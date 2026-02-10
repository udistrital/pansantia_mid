package services

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/internal/clients"
	internaldto "github.com/udistrital/pasantia_mid/internal/dto"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	rootservices "github.com/udistrital/pasantia_mid/services"

	"github.com/beego/beego/v2/server/web/context"
)

type visitaDTO struct {
	TutorId      int       `json:"tutor_id"`
	Total        int       `json:"total"`
	UltimaVisita time.Time `json:"ultima_visita"`
}

const estudiantePerfilResource = "estudiante_perfil"

// ConsultarPerfilPorDocumento identifica si existe un perfil asociado al número de documento dado.
func ConsultarPerfilPorDocumento(ctx *context.Context, numeroDocumento string) (*internaldto.EstudiantePerfilDocumentoResp, error) {
	numero := strings.TrimSpace(numeroDocumento)

	if numero == "" {
		return nil, helpers.NewAppError(http.StatusBadRequest, "numero_documento requerido", nil)
	}

	resp := &internaldto.EstudiantePerfilDocumentoResp{
		Relacionado: false,
		Mensaje:     "El estudiante no está registrado en Castor",
	}
	fmt.Println("Segunda Consulta Documento", numero)

	terceroID, err := rootservices.FindTerceroIDByDocumento(numero)
	fmt.Println("Respuesta Terceros 9999", terceroID)
	if err != nil {
		return nil, err
	}
	if terceroID == 0 {
		resp.Mensaje = "El documento no se encuentra registrado en terceros"
		return resp, nil
	}

	resp.TerceroID = &terceroID

	record, err := findPerfil(terceroID)
	fmt.Println("Record Data en Castor", record)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return resp, nil
	}

	perfilID := record.Id
	resp.Relacionado = true
	resp.PerfilID = &perfilID
	resp.Mensaje = "Estudiante registrado en Castor"
	fmt.Println("Respuesta Terceros Record", record)

	perfilMap := mapPerfil(*record)

	//Nombre completo desde Terceros/Core (tolerante)
	stdCtx := requestContext(ctx) // convierte *beego/context.Context -> context.Context
	if nombre := strings.TrimSpace(NombreCompletoPorIDCoreStd(stdCtx, terceroID)); nombre != "" {
		perfilMap["nombre_completo"] = nombre
	}

	fmt.Println("RecordPErfil", record)
	if nombre, err := obtenerNombreProyecto(ctx, record.ProyectoCurricularId); err == nil {
		fmt.Println("Nombre_PRoyecto", nombre)

		if proyectoNombre := strings.TrimSpace(nombre); proyectoNombre != "" {
			perfilMap["proyecto_curricular_nombre"] = proyectoNombre
			perfilMap["proyecto_curricular"] = map[string]interface{}{
				"id":     record.ProyectoCurricularId,
				"nombre": proyectoNombre,
			}
		}
	}

	resp.Perfil = perfilMap
	return resp, nil
}

// ObtenerPerfil trae el perfil del estudiante asociado al tercero.
func ObtenerPerfil(ctx *context.Context, terceroID int) (map[string]interface{}, error) {
	record, err := findPerfil(terceroID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "perfil no encontrado", nil)
	}

	perfilMap := mapPerfil(*record)

	//1) Nombre completo desde Terceros/Core (tolerante)
	stdCtx := requestContext(ctx) // <-- convierte *web/context.Context -> context.Context
	if nombre := strings.TrimSpace(NombreCompletoPorIDCoreStd(stdCtx, terceroID)); nombre != "" {
		perfilMap["nombre_completo"] = nombre
	}

	//enriquecer con nombre del proyecto curricular
	if nombre, err := obtenerNombreProyecto(ctx, record.ProyectoCurricularId); err == nil {
		if proyectoNombre := strings.TrimSpace(nombre); proyectoNombre != "" {
			perfilMap["proyecto_curricular_nombre"] = proyectoNombre
			perfilMap["proyecto_curricular"] = map[string]interface{}{
				"id":     record.ProyectoCurricularId,
				"nombre": proyectoNombre,
			}
		}
	}

	return perfilMap, nil
}

// UpsertPerfil crea o actualiza el perfil según exista previamente.
func UpsertPerfil(ctx *context.Context, terceroID int, payload internaldto.EstudiantePerfilUpsert) (map[string]interface{}, error) {
	if payload.ProyectoCurricularID == nil || *payload.ProyectoCurricularID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "proyecto_curricular_id es requerido", nil)
	}

	// Obligatorio: convertir código Académica -> id_oikos antes de persistir
	idOikos, _, err := HomologarAcademicaToOikos(ctx, *payload.ProyectoCurricularID)
	if err != nil {
		return nil, err // obligatorio => se rechaza
	}
	*payload.ProyectoCurricularID = idOikos

	if err := validarCvDocumentoID(ctx, payload.CVDocumentoID); err != nil {
		return nil, err
	}

	record, err := findPerfil(terceroID)
	if err != nil {
		return nil, err
	}

	if record == nil {
		return crearPerfil(terceroID, payload)
	}

	return actualizarPerfil(record.Id, payload)
}

// ActualizarPerfil actualiza parcialmente el perfil existente del estudiante.
func ActualizarPerfil(ctx *context.Context, terceroID int, payload internaldto.EstudiantePerfilUpsert) (map[string]interface{}, error) {
	record, err := findPerfil(terceroID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "perfil no encontrado", nil)
	}

	if payload.Visible != nil {
		stdCtx := requestContext(ctx)
		if ok, _ := isPasanteActivo(stdCtx, terceroID); ok {
			return nil, helpers.NewAppError(http.StatusConflict,
				"No puedes cambiar la visibilidad mientras tengas una pasantía activa", nil)
		}
	}

	if payload.ProyectoCurricularID != nil {
		if *payload.ProyectoCurricularID <= 0 {
			return nil, helpers.NewAppError(http.StatusBadRequest, "proyecto_curricular_id inválido", nil)
		}
		idOikos, _, err := HomologarAcademicaToOikos(ctx, *payload.ProyectoCurricularID)
		if err != nil {
			return nil, helpers.NewAppError(http.StatusBadRequest, "proyecto_curricular_id no homologable", err)
		}
		*payload.ProyectoCurricularID = idOikos
	}

	if err := validarCvDocumentoID(ctx, payload.CVDocumentoID); err != nil {
		return nil, err
	}

	if payload.ProyectoCurricularID == nil &&
		payload.Resumen == nil &&
		payload.Habilidades == nil &&
		payload.CVDocumentoID == nil &&
		payload.Visible == nil &&
		payload.TratamientoDatosAceptado == nil {
		return mapPerfil(*record), nil
	}

	fmt.Println("PAYLOAD VISIBILIDAD", payload)
	return actualizarPerfil(record.Id, payload)
}

// ListarVisitasPorEstudiante resume las visitas al perfil agrupadas por tutor.
func ListarVisitasPorEstudiante(ctx *context.Context, estudianteID int) (map[string]interface{}, error) {
	stdCtx := requestContext(ctx)
	crud := clients.CastorCRUD()

	perfil, err := crud.GetPerfilByTerceroID(stdCtx, estudianteID)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando perfil")
	}
	if perfil == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "perfil no encontrado", nil)
	}

	visitas, err := crud.ListPerfilVisitas(stdCtx, perfil.Id)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando visitas")
	}

	totalVisitas := len(visitas)
	agrupado := map[int]*visitaResumen{}

	for _, visita := range visitas {
		if visita.TutorId == 0 {
			continue
		}
		entry, ok := agrupado[visita.TutorId]
		if !ok {
			entry = &visitaResumen{TutorID: visita.TutorId}
			agrupado[visita.TutorId] = entry
		}
		entry.Total++
		if visita.FechaVisita.After(entry.Ultima) {
			entry.Ultima = visita.FechaVisita
		} else if entry.Ultima.IsZero() {
			entry.Ultima = visita.FechaVisita
		}
	}

	items := make([]visitaResumen, 0, len(agrupado))
	for _, v := range agrupado {
		items = append(items, *v)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Ultima.After(items[j].Ultima)
	})

	payload := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		entry := map[string]interface{}{
			"tutor_id": item.TutorID,
			"total":    item.Total,
		}
		if !item.Ultima.IsZero() {
			entry["ultima_visita"] = item.Ultima.Format(time.RFC3339)
		}
		payload = append(payload, entry)
	}

	return map[string]interface{}{
		"items": payload,
		"total": totalVisitas,
	}, nil
}

type visitaResumen struct {
	TutorID int
	Total   int
	Ultima  time.Time
}

func crearPerfil(terceroID int, payload internaldto.EstudiantePerfilUpsert) (map[string]interface{}, error) {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, estudiantePerfilResource)

	body := map[string]interface{}{
		"TerceroId":                terceroID,
		"ProyectoCurricularId":     *payload.ProyectoCurricularID,
		"FechaCreacion":            nowISO(),
		"FechaModificacion":        nowISO(),
		"Visible":                  true,
		"TratamientoDatosAceptado": false,
	}

	if payload.Visible != nil {
		body["Visible"] = *payload.Visible
	}
	if payload.TratamientoDatosAceptado != nil {
		body["TratamientoDatosAceptado"] = *payload.TratamientoDatosAceptado
	}
	if payload.Resumen != nil {
		body["Resumen"] = strings.TrimSpace(*payload.Resumen)
	}
	if payload.Habilidades != nil {
		body["Habilidades"] = strings.TrimSpace(*payload.Habilidades)
	}
	if payload.CVDocumentoID != nil {
		body["CvDocumentoId"] = *payload.CVDocumentoID
	}

	var created perfilRecord
	if err := helpers.DoJSON("POST", endpoint, body, &created, cfg.RequestTimeout); err != nil {
		return nil, helpers.AsAppError(err, "error creando perfil de estudiante")
	}
	return mapPerfil(created), nil
}

func actualizarPerfil(perfilID int, payload internaldto.EstudiantePerfilUpsert) (map[string]interface{}, error) {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, estudiantePerfilResource, fmt.Sprintf("%d", perfilID))

	body := map[string]interface{}{
		"FechaModificacion": nowISO(),
	}

	if payload.ProyectoCurricularID != nil && *payload.ProyectoCurricularID > 0 {
		body["ProyectoCurricularId"] = *payload.ProyectoCurricularID
	}
	if payload.Resumen != nil {
		body["Resumen"] = strings.TrimSpace(*payload.Resumen)
	}
	if payload.Habilidades != nil {
		body["Habilidades"] = strings.TrimSpace(*payload.Habilidades)
	}
	if payload.CVDocumentoID != nil {
		body["CvDocumentoId"] = *payload.CVDocumentoID
	}
	if payload.Visible != nil {
		body["Visible"] = *payload.Visible
	}
	if payload.TratamientoDatosAceptado != nil {
		body["TratamientoDatosAceptado"] = *payload.TratamientoDatosAceptado
	}

	var updated perfilRecord
	fmt.Println("BODY CONSULTA PUT VISIBILIDAD", body)
	if err := helpers.DoJSON("PUT", endpoint, body, &updated, cfg.RequestTimeout); err != nil {
		return nil, helpers.AsAppError(err, "error actualizando perfil de estudiante")
	}
	return mapPerfil(updated), nil
}

func findPerfil(terceroID int) (*perfilRecord, error) {
	fmt.Println("Revisión de cfg", terceroID)
	cfg := rootservices.GetConfig()

	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, estudiantePerfilResource)
	values := url.Values{}
	values.Set("limit", "1")
	values.Set("query", fmt.Sprintf("TerceroId:%d", terceroID))

	urlWithQuery := endpoint
	if query := values.Encode(); query != "" {
		urlWithQuery = endpoint + "?" + query
	}

	fmt.Println("URL Consulta Estudiante_perfil", urlWithQuery)
	headers := rootservices.AddOASAuth(nil)
	var records []perfilRecord
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &records, cfg.RequestTimeout, true); err != nil {
		if helpers.IsHTTPError(err, http.StatusNotFound) {
			return nil, nil
		}
		return nil, helpers.AsAppError(err, "error consultando perfil de estudiante")
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

func mapPerfil(record perfilRecord) map[string]interface{} {
	result := map[string]interface{}{
		"id":                         record.Id,
		"tercero_id":                 record.TerceroId,
		"proyecto_curricular_id":     record.ProyectoCurricularId,
		"resumen":                    strings.TrimSpace(record.Resumen),
		"habilidades":                normalizeHabilidades(record.Habilidades),
		"visible":                    record.Visible,
		"tratamiento_datos_aceptado": record.TratamientoDatosAceptado,
		"fecha_creacion":             strings.TrimSpace(record.FechaCreacion),
		"fecha_modificacion":         strings.TrimSpace(record.FechaModificacion),
	}
	if doc := decodeCvDocumento(record.CvDocumentoRaw); doc != nil {
		result["cv_documento_id"] = doc
	}
	return result
}

func validarCvDocumentoID(ctx *context.Context, doc *string) error {
	if doc == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*doc)
	if trimmed == "" {
		return helpers.NewAppError(http.StatusBadRequest, "cv_documento_id inválido", nil)
	}
	*doc = trimmed

	ok, err := internalhelpers.Documentos.Exists(ctx, trimmed)
	if err != nil {
		return err
	}
	if !ok {
		return helpers.NewAppError(http.StatusBadRequest, "cv_documento_id no existe", nil)
	}
	return nil
}

func obtenerNombreProyecto(ctx *context.Context, id int) (string, error) {
	if ctx == nil || id <= 0 {
		return "", nil
	}

	// 1)Intento principal: por Oikos (mismo enfoque que en explorar_service.go)
	if detalle, err := rootservices.GetProyectoCurricular(id); err == nil && detalle != nil {
		if n := strings.TrimSpace(detalle.Nombre); n != "" {
			return n, nil
		}
	}

	// 2) Fallback
	if nombre, err := ObtenerNombreProyectoCurricular(ctx, id); err == nil {
		if n := strings.TrimSpace(nombre); n != "" {
			return n, nil
		}
	} else {

	}

	// 3) Último intento: si "id" venía como código Académica, homologar a Oikos y volver a intentar
	idOikos, _, err := HomologarAcademicaToOikos(ctx, id)
	if err == nil && idOikos > 0 && idOikos != id {
		if detalle, err2 := rootservices.GetProyectoCurricular(idOikos); err2 == nil && detalle != nil {
			if n := strings.TrimSpace(detalle.Nombre); n != "" {
				return n, nil
			}
		}
		if nombre, err2 := ObtenerNombreProyectoCurricular(ctx, idOikos); err2 == nil {
			if n := strings.TrimSpace(nombre); n != "" {
				return n, nil
			}
		}
	}

	return "", nil
}

func normalizeHabilidades(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var parsed interface{}
	if json.Unmarshal([]byte(trimmed), &parsed) == nil {
		return parsed
	}
	return trimmed
}

type perfilRecord struct {
	Id                       int             `json:"Id"`
	TerceroId                int             `json:"TerceroId"`
	ProyectoCurricularId     int             `json:"ProyectoCurricularId"`
	Resumen                  string          `json:"Resumen"`
	Habilidades              string          `json:"Habilidades"`
	CvDocumentoRaw           json.RawMessage `json:"CvDocumentoId"`
	Visible                  bool            `json:"Visible"`
	TratamientoDatosAceptado bool            `json:"TratamientoDatosAceptado"`
	FechaCreacion            string          `json:"FechaCreacion"`
	FechaModificacion        string          `json:"FechaModificacion"`
}

func decodeCvDocumento(raw json.RawMessage) interface{} {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var intVal int
	if err := json.Unmarshal(raw, &intVal); err == nil {
		return intVal
	}
	var strVal string
	if err := json.Unmarshal(raw, &strVal); err == nil {
		return strings.TrimSpace(strVal)
	}
	return nil
}

func normalizarProyectoCurricularID(ctx *context.Context, id int) (int, error) {
	if id <= 0 {
		return 0, nil
	}

	// 1) Si existe en Oikos, asumimos que YA es id_oikos.
	if nombre, _ := ObtenerNombreProyectoCurricular(ctx, id); strings.TrimSpace(nombre) != "" {
		return id, nil
	}

	// 2) Si no existe en Oikos, lo tratamos como código Académica y homologamos.
	idOikos, _, err := HomologarAcademicaToOikos(ctx, id) // ajusta import/nombre según dónde lo pongas
	if err != nil {
		return 0, err
	}
	return idOikos, nil
}

func isPasanteActivo(stdCtx stdctx.Context, estudianteID int) (bool, map[string]interface{}) {
	if estudianteID <= 0 {
		return false, nil
	}

	list, err := clients.CastorCRUD().ListPostulaciones(stdCtx, map[string]string{
		"EstudianteId": fmt.Sprint(estudianteID),
	})
	if err != nil || len(list) == 0 {
		return false, nil
	}

	for _, post := range list {
		estadoPost := strings.ToUpper(strings.TrimSpace(post.EstadoPostulacion))
		if estadoPost != "PSAC_CTR" {
			continue
		}

		oferta, err := rootservices.GetOferta(post.OfertaId)
		if err != nil || oferta == nil {
			continue
		}

		estadoOferta := strings.ToUpper(strings.TrimSpace(oferta.Estado))
		if estadoOferta != "OPCUR_CTR" {
			continue
		}

		return true, map[string]interface{}{
			"oferta_id":     post.OfertaId,
			"titulo_oferta": strings.TrimSpace(oferta.Titulo),
		}
	}

	return false, nil
}
