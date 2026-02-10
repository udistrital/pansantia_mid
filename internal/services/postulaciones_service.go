package services

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/internal/clients"
	internaldto "github.com/udistrital/pasantia_mid/internal/dto"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	"github.com/udistrital/pasantia_mid/models"
	rootservices "github.com/udistrital/pasantia_mid/services"
)

const (
	postulacionRevisionResource = "postulacion_revision"

	accionVisto          = "VISTO"
	accionDescartar      = "DESCARTAR"
	accionPreseleccionar = "PRESELECCIONAR"
	accionSeleccionar    = "SELECCIONAR"

	preseleccionEnvKey = "POSTULACION_PRESELECCION_CODIGO"
	defaultPreselect   = "PSPR_CTR"
)

const (
	estadoSeleccionado         = "SELECCIONADO"
	estadoSeleccionarLegacy    = "SELECCIONAR"
	estadoAceptadaEstudiante   = "ACEPTADA_POR_ESTUDIANTE"
	estadoRechazadaPorEleccion = "RECHAZADA_POR_ELECCION"

	accionRevisionAceptar     = "ACEPTAR_SELECCION"
	accionRevisionRechazo     = "RECHAZAR_POR_ELECCION"
	comentarioRevisionAceptar = "Aceptada por estudiante"
	comentarioRevisionRechazo = "Rechazada por elección de otra oferta"
)

var (
	preselectOnce sync.Once
	preselectCode string
)

type estudianteEnriq struct {
	NombreCompleto           string
	ProyectoCurricularID     int
	ProyectoCurricularNombre string
}

// ListarPostulaciones trae las postulaciones de la oferta marcando si fueron vistas.
func ListarPostulaciones(ctx context.Context, tutorID, ofertaID int) (map[string]interface{}, error) {
	filters := map[string]string{
		"oferta_id": strconv.Itoa(ofertaID),
		"limit":     "0",
	}
	postulaciones, err := clients.CastorCRUD().ListPostulaciones(ctx, filters)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando postulaciones")
	}

	if len(postulaciones) == 0 {
		return map[string]interface{}{
			"items": []map[string]interface{}{},
			"total": 0,
		}, nil
	}

	postulacionIDs := make([]int64, 0, len(postulaciones))
	estudianteIDs := make([]int64, 0, len(postulaciones))
	for _, p := range postulaciones {
		postulacionIDs = append(postulacionIDs, p.Id)
		estudianteIDs = append(estudianteIDs, p.EstudianteId)
	}

	vistos := traerRevisiones(tutorID, postulacionIDs)
	cvMap := traerCvDocumentos(estudianteIDs)
	enriq := enriquecerEstudiantes(ctx, estudianteIDs)

	items := make([]map[string]interface{}, 0, len(postulaciones))
	for _, p := range postulaciones {
		code := strings.ToUpper(strings.TrimSpace(p.EstadoPostulacion))
		estadoNombre := resolveEstadoNombre(code)
		info := enriq[p.EstudianteId]
		item := map[string]interface{}{
			"id":                         p.Id,
			"estudiante_id":              p.EstudianteId,
			"oferta_id":                  p.OfertaId,
			"estado":                     strings.TrimSpace(p.EstadoPostulacion),
			"fecha_postulacion":          strings.TrimSpace(p.FechaPostulacion),
			"enlace_doc_hv":              strings.TrimSpace(p.EnlaceDocHv),
			"visto":                      vistos[p.Id],
			"estudiante_nombre":          info.NombreCompleto,
			"proyecto_curricular_id":     info.ProyectoCurricularID,
			"proyecto_curricular_nombre": info.ProyectoCurricularNombre,
			"estudiante": map[string]any{
				"Id":             p.EstudianteId,
				"NombreCompleto": info.NombreCompleto,
			},
			"proyecto_curricular": map[string]any{
				"id":     info.ProyectoCurricularID,
				"nombre": info.ProyectoCurricularNombre,
			},
		}
		item["Estado"] = map[string]string{
			"code":   code,
			"nombre": estadoNombre,
		}
		if cv, ok := cvMap[p.EstudianteId]; ok {
			item["cv_documento_id"] = cv
		}
		items = append(items, item)
	}

	return map[string]interface{}{
		"items": items,
		"total": len(items),
	}, nil
}

// EjecutarAccionPostulacion registra la acción y actualiza estado según corresponda.
func EjecutarAccionPostulacion(ctx context.Context, tutorID int, postulacionID int64, payload internaldto.PostulacionAccion) (map[string]interface{}, error) {
	crud := clients.CastorCRUD()
	accion := strings.ToUpper(strings.TrimSpace(payload.Accion))
	if !accionValida(accion) {
		return nil, helpers.NewAppError(http.StatusBadRequest, "accion no soportada", nil)
	}

	postulacion, err := crud.GetPostulacionByID(ctx, postulacionID)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando postulacion")
	}
	if postulacion == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "postulacion no encontrada", nil)
	}

	oferta, err := rootservices.GetOferta(postulacion.OfertaId)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando oferta asociada")
	}
	if oferta == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "oferta asociada no encontrada", nil)
	}
	if int(oferta.TutorExternoId) != tutorID {
		return nil, helpers.NewAppError(http.StatusForbidden, "no autorizado para gestionar esta postulación", nil)
	}

	estadoActual := strings.ToUpper(strings.TrimSpace(postulacion.EstadoPostulacion))
	if accion == accionDescartar && estadoActual == models.PostEstadoRechazada {
		return nil, helpers.NewAppError(http.StatusConflict, "La postulación ya está descartada", nil)
	}
	if (accion == accionDescartar || accion == accionPreseleccionar) && estadoFinal(estadoActual) {
		return nil, helpers.NewAppError(http.StatusConflict, "La postulación está en estado final", nil)
	}

	switch accion {
	case accionVisto:
		if estadoActual == models.PostEstadoPorRevisar {
			if err = crud.UpdatePostulacionEstado(ctx, postulacionID, models.PostEstadoRevisada, time.Now().UTC()); err != nil {
				return nil, err
			}
		}
	case accionDescartar:
		if err := registrarRevision(ctx, tutorID, postulacionID, accion, payload.Comentario); err != nil {
			return nil, err
		}
		if _, err = rootservices.DescartarPostulacion(postulacionID); err != nil {
			return nil, err
		}
	case accionSeleccionar:
		if err := registrarRevision(ctx, tutorID, postulacionID, accion, payload.Comentario); err != nil {
			return nil, err
		}
		if _, err = rootservices.SeleccionarPostulacion(postulacionID); err != nil {
			return nil, err
		}
	case accionPreseleccionar:
		if err := registrarRevision(ctx, tutorID, postulacionID, accion, payload.Comentario); err != nil {
			return nil, err
		}
		code := obtenerPreselectCodigo()
		if err = crud.UpdatePostulacionEstado(ctx, postulacionID, code, time.Now().UTC()); err != nil {
			return nil, err
		}
	default:
	}

	updated, err := crud.GetPostulacionByID(ctx, postulacionID)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando postulación actualizada")
	}
	if updated == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "postulación no encontrada", nil)
	}

	response := map[string]interface{}{
		"id":                updated.Id,
		"estudiante_id":     updated.EstudianteId,
		"oferta_id":         updated.OfertaId,
		"estado":            strings.TrimSpace(updated.EstadoPostulacion),
		"fecha_postulacion": strings.TrimSpace(updated.FechaPostulacion),
		"accion":            accion,
	}
	code := strings.ToUpper(strings.TrimSpace(updated.EstadoPostulacion))
	response["estado_det"] = map[string]string{
		"code":   code,
		"nombre": resolveEstadoNombre(code),
	}

	return response, nil
}

func estadoFinal(estado string) bool {
	switch strings.ToUpper(strings.TrimSpace(estado)) {
	case models.PostEstadoSeleccionada,
		models.PostEstadoAceptada,
		models.PostEstadoDescartada:
		return true
	}
	return false
}

func resolveEstadoNombre(code string) string {
	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "" {
		return c
	}
	fallback := map[string]string{
		"PSPO_CTR": "Postulada",
		"PSRV_CTR": "En revisión",
		"PSPR_CTR": "Preseleccionada",
		"PSSE_CTR": "Seleccionada",
		"PSRJ_CTR": "Descartada",
	}
	if c != "" {
		if par, err := internalhelpers.GetParametroByCodeNoCache(c); err == nil {
			if nombre := strings.TrimSpace(par.Nombre); nombre != "" {
				return nombre
			}
		}
	}
	if nombre, ok := fallback[c]; ok {
		return nombre
	}
	return c
}

// MarcarPostulacionVista marca una postulación como revisada (PSRV_CTR) si está en PSPO_CTR.
// Si ya está en otro estado, devuelve la postulación sin cambios (idempotente).
func MarcarPostulacionVista(ctx context.Context, tutorID int, postulacionID int64) (map[string]interface{}, error) {
	crud := clients.CastorCRUD()
	postulacion, err := crud.GetPostulacionByID(ctx, postulacionID)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando postulacion")
	}
	if postulacion == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "postulacion no encontrada", nil)
	}

	oferta, err := rootservices.GetOferta(postulacion.OfertaId)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando oferta asociada")
	}
	if oferta == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "oferta asociada no encontrada", nil)
	}
	if int(oferta.TutorExternoId) != tutorID {
		return nil, helpers.NewAppError(http.StatusForbidden, "no autorizado para gestionar esta postulación", nil)
	}

	estadoActual := strings.ToUpper(strings.TrimSpace(postulacion.EstadoPostulacion))
	if estadoActual == "PSPO_CTR" {
		if err := crud.UpdatePostulacionEstado(ctx, postulacionID, "PSRV_CTR", time.Now().UTC()); err != nil {
			return nil, helpers.AsAppError(err, "error actualizando estado de postulación")
		}
		if actualizado, err := crud.GetPostulacionByID(ctx, postulacionID); err == nil && actualizado != nil {
			postulacion = actualizado
		}
	}

	return map[string]interface{}{
		"id":                postulacion.Id,
		"estudiante_id":     postulacion.EstudianteId,
		"oferta_id":         postulacion.OfertaId,
		"estado":            strings.TrimSpace(postulacion.EstadoPostulacion),
		"fecha_postulacion": strings.TrimSpace(postulacion.FechaPostulacion),
	}, nil
}

// AceptarSeleccion aplica la elección única del estudiante cuando tiene varias postulaciones seleccionadas.
// func AceptarSeleccion(ctx context.Context, estudianteID int, postulacionID int64) error {
// 	stdCtx := requestContext(ctx)
// 	crud := clients.CastorCRUD()

// 	postulacion, err := crud.GetPostulacionByID(stdCtx, postulacionID)
// 	if err != nil {
// 		return helpers.AsAppError(err, "postulación no encontrada")
// 	}
// 	if postulacion == nil {
// 		return helpers.NewAppError(http.StatusNotFound, "postulación no encontrada", nil)
// 	}
// 	if int(postulacion.EstudianteId) != estudianteID {
// 		return helpers.NewAppError(http.StatusForbidden, "la postulación no pertenece al estudiante", nil)
// 	}

// 	currentState := strings.ToUpper(strings.TrimSpace(postulacion.EstadoPostulacion))
// 	if currentState != estadoSeleccionado && currentState != estadoSeleccionarLegacy {
// 		return helpers.NewAppError(http.StatusConflict, "la postulación no está en estado SELECCIONADO", nil)
// 	}

// 	now := time.Now().UTC()
// 	if err := crud.UpdatePostulacionEstado(stdCtx, postulacion.Id, estadoAceptadaEstudiante, now); err != nil {
// 		return helpers.AsAppError(err, "no fue posible aceptar la selección")
// 	}
// 	_ = crud.AddPostulacionRevision(stdCtx, postulacion.Id, 0, accionRevisionAceptar, comentarioRevisionAceptar, now)

//		filters := map[string]string{
//			"EstudianteId":              strconv.Itoa(estudianteID),
//			"Id__ne":                    strconv.FormatInt(postulacion.Id, 10),
//			"EstadoPostulacion__iexact": estadoSeleccionado,
//		}
//		if others, err := crud.ListPostulaciones(stdCtx, filters); err == nil {
//			for _, other := range others {
//				_ = crud.UpdatePostulacionEstado(stdCtx, other.Id, estadoRechazadaPorEleccion, now)
//				_ = crud.AddPostulacionRevision(stdCtx, other.Id, 0, accionRevisionRechazo, comentarioRevisionRechazo, now)
//			}
//		}
//		return nil
//	}
//
// AceptarSeleccion:
// - Verifica que la postulación pertenezca al estudiante y esté en SELECCIONADO
// - Marca esa postulación → ACEPTADA_POR_ESTUDIANTE (con FechaEstado)
// - Todas las demás del mismo estudiante en SELECCIONADO → RECHAZADA_POR_ELECCION
// - Registra revisiones en postulacion_revision
func AceptarSeleccion(ctx context.Context, estudianteID int, postulacionID int64) error {
	crud := clients.CastorCRUD()

	// 1) Obtener la postulación
	post, err := crud.GetPostulacionByID(ctx, postulacionID)
	if err != nil {
		return helpers.NewAppError(http.StatusNotFound, "postulación no encontrada", err)
	}
	if int(post.EstudianteId) != estudianteID {
		return helpers.NewAppError(http.StatusForbidden, "la postulación no pertenece al estudiante", nil)
	}
	estadoActual := strings.ToUpper(strings.TrimSpace(post.EstadoPostulacion))
	if estadoActual != "SELECCIONAR" && estadoActual != "SELECCIONADO" && estadoActual != models.PostEstadoSeleccionada {
		return helpers.NewAppError(http.StatusConflict, "la postulación no está en estado SELECCIONADO", nil)
	}

	now := time.Now().UTC()

	// 2) Aceptar esta postulación
	if err := crud.UpdatePostulacionEstado(ctx, int64(post.Id), models.PostEstadoAceptada, now); err != nil {
		return helpers.NewAppError(http.StatusInternalServerError, "no fue posible aceptar la selección", err)
	}
	_ = crud.AddPostulacionRevision(ctx, int64(post.Id), 0, "ACEPTAR_SELECCION", "Aceptada por el estudiante", now)

	// 3) Rechazar por elección las otras SELECCIONADAS del mismo estudiante
	others, err := crud.ListPostulaciones(ctx, map[string]string{
		"EstudianteId": fmt.Sprint(estudianteID),
		"Id__ne":       fmt.Sprint(post.Id), // por si no soporta __ne, filtramos en memoria abajo
		"limit":        "0",
	})
	if err == nil {
		for _, p := range others {
			if p.Id == post.Id {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(p.EstadoPostulacion), "SELECCIONADO") ||
				strings.EqualFold(strings.TrimSpace(p.EstadoPostulacion), models.PostEstadoSeleccionada) {
				_ = crud.UpdatePostulacionEstado(ctx, int64(p.Id), "RECHAZADA_POR_ELECCION", now)
				_ = crud.AddPostulacionRevision(ctx, int64(p.Id), 0, "RECHAZAR_POR_ELECCION", "Rechazada por elección de otra oferta", now)
			}
		}
	}

	return nil
}

func accionValida(accion string) bool {
	switch accion {
	case accionVisto, accionDescartar, accionPreseleccionar, accionSeleccionar:
		return true
	default:
		return false
	}
}

func registrarRevision(ctx context.Context, tutorID int, postulacionID int64, accion string, comentario *string) error {
	var note string
	if comentario != nil {
		note = strings.TrimSpace(*comentario)
	}
	if err := clients.CastorCRUD().AddPostulacionRevision(ctx, postulacionID, tutorID, accion, note, time.Now().UTC()); err != nil {
		return helpers.AsAppError(err, "error registrando revision de postulacion")
	}
	return nil
}

func traerRevisiones(tutorID int, postulacionIDs []int64) map[int64]bool {
	result := make(map[int64]bool, len(postulacionIDs))
	if len(postulacionIDs) == 0 {
		return result
	}
	idSet := make(map[int64]struct{}, len(postulacionIDs))
	for _, id := range postulacionIDs {
		idSet[id] = struct{}{}
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, postulacionRevisionResource)

	values := url.Values{}
	values.Set("limit", "0")
	values.Set("query", fmt.Sprintf("TutorId:%d", tutorID))

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var revisiones []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &revisiones, cfg.RequestTimeout); err != nil {
		return result
	}

	for _, rev := range revisiones {
		if id, ok := normalizeToInt64(rev["PostulacionId"]); ok {
			if _, exists := idSet[id]; exists {
				result[id] = true
			}
		}
	}
	return result
}

func traerCvDocumentos(estudianteIDs []int64) map[int64]string {
	result := make(map[int64]string)
	if len(estudianteIDs) == 0 {
		return result
	}

	unique := make(map[int64]struct{}, len(estudianteIDs))
	for _, id := range estudianteIDs {
		unique[id] = struct{}{}
	}

	cfg := rootservices.GetConfig()
	for id := range unique {
		values := url.Values{}
		values.Set("limit", "1")
		values.Set("query", fmt.Sprintf("TerceroId:%d", id))

		endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, "estudiante_perfil")
		urlWithQuery := endpoint
		if encoded := values.Encode(); encoded != "" {
			urlWithQuery = endpoint + "?" + encoded
		}

		var perfiles []map[string]interface{}
		if err := helpers.DoJSON("GET", urlWithQuery, nil, &perfiles, cfg.RequestTimeout); err != nil {
			continue
		}
		if len(perfiles) == 0 {
			continue
		}
		if cv := strings.TrimSpace(normalizeToString(perfiles[0]["CvDocumentoId"])); cv != "" {
			result[id] = cv
		}
	}
	return result
}

func obtenerPreselectCodigo() string {
	preselectOnce.Do(func() {
		code := strings.TrimSpace(os.Getenv(preseleccionEnvKey))
		if code == "" {
			code = defaultPreselect
		}
		preselectCode = strings.ToUpper(code)
	})
	return preselectCode
}

func enriquecerEstudiantes(ctx context.Context, estudianteIDs []int64) map[int64]estudianteEnriq {
	result := make(map[int64]estudianteEnriq)
	if len(estudianteIDs) == 0 {
		return result
	}

	unique := make(map[int64]struct{}, len(estudianteIDs))
	for _, id := range estudianteIDs {
		if id > 0 {
			unique[id] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return result
	}

	var mu sync.Mutex
	pcNameCache := make(map[int]string)
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for id := range unique {
		wg.Add(1)
		sem <- struct{}{}
		go func(estID int64) {
			defer wg.Done()
			defer func() { <-sem }()

			info := estudianteEnriq{}

			nombre := NombreCompletoPorIDCoreStd(ctx, int(estID))
			info.NombreCompleto = nombre

			if perfil, err := clients.CastorCRUD().GetPerfilByTerceroID(ctx, int(estID)); err == nil && perfil != nil {
				if perfil.ProyectoCurricularId > 0 {
					info.ProyectoCurricularID = perfil.ProyectoCurricularId

					mu.Lock()
					pcNombre, ok := pcNameCache[perfil.ProyectoCurricularId]
					mu.Unlock()

					if !ok {
						if pc, err := rootservices.GetProyectoCurricular(perfil.ProyectoCurricularId); err == nil && pc != nil {
							pcNombre = strings.TrimSpace(pc.Nombre)
						}
						mu.Lock()
						pcNameCache[perfil.ProyectoCurricularId] = pcNombre
						mu.Unlock()
					}

					info.ProyectoCurricularNombre = pcNombre
				}
			}

			mu.Lock()
			result[estID] = info
			mu.Unlock()
		}(id)
	}

	wg.Wait()
	return result
}
