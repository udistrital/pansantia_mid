package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/internal/clients"
	internaldto "github.com/udistrital/pasantia_mid/internal/dto"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	rootmodels "github.com/udistrital/pasantia_mid/models"
	rootservices "github.com/udistrital/pasantia_mid/services"
)

const (
	//CRUD real (manual): /v1/invitaciones
	invitacionesResource = "invitaciones"

	InvitacionEstadoEnviada   = "ENVIADA"
	InvitacionEstadoAceptada  = "ACEPTADA"
	InvitacionEstadoRechazada = "RECHAZADA"
)

// CrearInvitacion -> CRUD: POST /v1/invitaciones/perfil/:perfil_id (requiere header X-Tutor-Id)
func CrearInvitacion(ctx context.Context, tutorID, perfilID int, payload internaldto.InvitacionCreate) (map[string]interface{}, error) {
	_ = ctx

	if tutorID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "tutor_id inválido", nil)
	}
	if perfilID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "perfil_id inválido", nil)
	}
	if strings.TrimSpace(payload.Mensaje) == "" {
		return nil, helpers.NewAppError(http.StatusBadRequest, "mensaje es requerido", nil)
	}

	var ofertaID int64
	if payload.OfertaPasantiaID != nil && *payload.OfertaPasantiaID > 0 {
		ofertaID = *payload.OfertaPasantiaID
	} else if payload.OfertaID != nil && *payload.OfertaID > 0 {
		ofertaID = *payload.OfertaID
	}
	if ofertaID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "oferta_id/oferta_pasantia_id es requerido", nil)
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, invitacionesResource, "perfil", strconv.Itoa(perfilID))

	// El CRUD espera {mensaje, oferta_id}
	body := map[string]interface{}{
		"mensaje":   strings.TrimSpace(payload.Mensaje),
		"oferta_id": int(ofertaID),
	}

	var created map[string]interface{}
	if err := helpers.DoJSONWithHeaders(
		"POST",
		endpoint,
		map[string]string{"X-Tutor-Id": fmt.Sprint(tutorID)},
		body,
		&created,
		cfg.RequestTimeout,
		true, // CRUD viene envuelto (success/status/message/data)
	); err != nil {
		return nil, helpers.AsAppError(err, "error creando invitación")
	}

	out := normalizeInvitacion(created)
	enrichInvitacionEstados([]map[string]interface{}{out})
	attachOfertaResumen([]map[string]interface{}{out})
	return out, nil
}

// BandejaTutor -> CRUD: GET /v1/invitaciones?estado=... + header X-Tutor-Id
func BandejaTutor(ctx context.Context, tutorID int, estado string, page, size int) (internaldto.PageDTO[map[string]interface{}], error) {
	_ = ctx

	if tutorID <= 0 {
		return internaldto.PageDTO[map[string]interface{}]{}, helpers.NewAppError(http.StatusBadRequest, "tutor_id inválido", nil)
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, invitacionesResource)

	values := url.Values{}
	if s := strings.TrimSpace(estado); s != "" {
		values.Set("estado", strings.ToUpper(s))
	}

	urlWithQuery := endpoint
	if enc := values.Encode(); enc != "" {
		urlWithQuery = endpoint + "?" + enc
	}

	var raw []map[string]interface{}
	if err := helpers.DoJSONWithHeaders(
		"GET",
		urlWithQuery,
		map[string]string{"X-Tutor-Id": fmt.Sprint(tutorID)},
		nil,
		&raw,
		cfg.RequestTimeout,
		true,
	); err != nil {
		return internaldto.PageDTO[map[string]interface{}]{}, helpers.AsAppError(err, "error consultando invitaciones")
	}

	items := make([]map[string]interface{}, 0, len(raw))
	for _, it := range raw {
		items = append(items, normalizeInvitacion(it))
	}

	// orden desc por fecha_creacion (string compare sirve si viene ISO)
	sort.SliceStable(items, func(i, j int) bool {
		return fmt.Sprint(items[i]["fecha_creacion"]) > fmt.Sprint(items[j]["fecha_creacion"])
	})

	enrichInvitacionEstados(items)
	attachOfertaResumen(items)
	enrichBandejaTutorConEstudiantes(ctx, items)

	// paginación manual (sin usar paginate global del package)
	total := len(items)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	paged := items[start:end]

	return internaldto.PageDTO[map[string]interface{}]{
		Items: paged,
		Page:  page,
		Size:  size,
		Total: int64(total),
	}, nil
}

// ListarInvitacionesDeEstudiante -> CRUD: GET /v1/invitaciones?estudiante_id=... [&estado=...]
func ListarInvitacionesDeEstudiante(ctx context.Context, estudianteID int, estado string, page, size int) (map[string]interface{}, error) {
	if estudianteID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "estudiante_id inválido", nil)
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, invitacionesResource)

	values := url.Values{}
	values.Set("estudiante_id", fmt.Sprint(estudianteID))
	if s := strings.TrimSpace(estado); s != "" {
		values.Set("estado", strings.ToUpper(s))
	}

	urlWithQuery := endpoint + "?" + values.Encode()

	var raw []map[string]interface{}
	// DoJSON => wrapped=true por defecto (ok porque CRUD responde wrapper)
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &raw, cfg.RequestTimeout); err != nil {
		return nil, helpers.AsAppError(err, "error consultando invitaciones")
	}

	items := make([]map[string]interface{}, 0, len(raw))
	for _, it := range raw {
		items = append(items, normalizeInvitacion(it))
	}

	sort.SliceStable(items, func(i, j int) bool {
		return fmt.Sprint(items[i]["fecha_creacion"]) > fmt.Sprint(items[j]["fecha_creacion"])
	})

	enrichInvitacionEstados(items)
	attachOfertaResumen(items)

	total := len(items)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	paged := items[start:end]

	return map[string]interface{}{
		"items": paged,
		"total": total,
		"page":  page,
		"size":  size,
	}, nil
}

// GetInvitacionDetalle retorna el detalle de una invitación validando acceso por tutor o estudiante.
func GetInvitacionDetalle(ctx context.Context, invitacionID int, tutorID int, estudianteID int, terceroID int) (map[string]interface{}, error) {
	if invitacionID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "id inválido", nil)
	}
	if estudianteID <= 0 && terceroID > 0 {
		estudianteID = terceroID
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, invitacionesResource, strconv.Itoa(invitacionID))

	findInTutorBandeja := func() (map[string]interface{}, error) {
		if tutorID <= 0 {
			return nil, nil
		}
		page := 1
		size := 1000
		bandeja, err := BandejaTutor(ctx, tutorID, "", page, size)
		if err != nil {
			return nil, err
		}
		for _, it := range bandeja.Items {
			id, _ := toInt(it["id"])
			if id == invitacionID {
				return it, nil
			}
		}
		return nil, nil
	}

	findInEstudianteBandeja := func() (map[string]interface{}, error) {
		if estudianteID <= 0 {
			return nil, nil
		}
		return findInvitacionInBandejaEstudiante(ctx, estudianteID, invitacionID)
	}

	var raw map[string]interface{}
	if err := helpers.DoJSON("GET", endpoint, nil, &raw, cfg.RequestTimeout); err == nil {
		inv := normalizeInvitacion(raw)
		if inv == nil {
			return nil, helpers.NewAppError(http.StatusNotFound, "invitación no encontrada", nil)
		}

		authorized := false
		if tutorID > 0 {
			if tid, ok := toInt(inv["tutor_id"]); ok && tid == tutorID {
				authorized = true
			}
		}

		if !authorized && estudianteID > 0 {
			if sid, ok := toInt(inv["estudiante_id"]); ok && sid > 0 {
				if sid == estudianteID {
					authorized = true
				}
			} else if found, err := findInEstudianteBandeja(); err != nil {
				return nil, err
			} else if found != nil {
				authorized = true
				inv = found
			}
		}

		if !authorized && tutorID > 0 {
			if tid, ok := toInt(inv["tutor_id"]); !ok || tid <= 0 {
				if found, err := findInTutorBandeja(); err != nil {
					return nil, err
				} else if found != nil {
					authorized = true
					inv = found
				}
			}
		}

		if !authorized {
			return nil, helpers.NewAppError(http.StatusForbidden, "no autorizado para consultar la invitación", nil)
		}

		enrichInvitacionEstados([]map[string]interface{}{inv})
		attachOfertaResumen([]map[string]interface{}{inv})
		enrichInvitacionDetalle(ctx, inv) // aquí sí
		enrichInvitacionConEstudianteDetalle(ctx, inv)
		return inv, nil
	} else if !helpers.IsHTTPError(err, http.StatusNotFound) && !helpers.IsHTTPError(err, http.StatusMethodNotAllowed) {
		return nil, helpers.AsAppError(err, "error consultando invitación")
	}

	// FALLBACKS
	if tutorID > 0 {
		if inv, err := findInTutorBandeja(); err != nil {
			return nil, err
		} else if inv != nil {
			enrichInvitacionEstados([]map[string]interface{}{inv})
			attachOfertaResumen([]map[string]interface{}{inv})
			enrichInvitacionDetalle(ctx, inv) // agregado
			enrichInvitacionConEstudianteDetalle(ctx, inv)
			return inv, nil
		}
	}
	if estudianteID > 0 {
		if inv, err := findInEstudianteBandeja(); err != nil {
			return nil, err
		} else if inv != nil {
			enrichInvitacionEstados([]map[string]interface{}{inv})
			attachOfertaResumen([]map[string]interface{}{inv})
			enrichInvitacionDetalle(ctx, inv) // agregado
			enrichInvitacionConEstudianteDetalle(ctx, inv)
			return inv, nil
		}
	}

	return nil, helpers.NewAppError(http.StatusNotFound, "invitación no encontrada", nil)
}

// AceptarInvitacion marca la invitación como aceptada.
func AceptarInvitacion(ctx context.Context, invitacionID int, terceroID int) (map[string]interface{}, error) {
	if invitacionID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "id inválido", nil)
	}

	fmt.Println("TERCERO_ID aCPETARiNVITACION", terceroID)
	if terceroID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "tercero_id inválido", nil)
	}

	inv, err := findInvitacionInBandejaEstudiante(ctx, terceroID, invitacionID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "invitación no encontrada", nil)
	}

	tutorID, _ := toInt(inv["tutor_id"])
	if tutorID <= 0 {
		return nil, helpers.NewAppError(http.StatusInternalServerError, "no fue posible determinar tutor_id de la invitación", nil)
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, invitacionesResource, strconv.Itoa(invitacionID), "aceptar")

	headers := map[string]string{
		"X-Tutor-Id":   fmt.Sprint(tutorID),
		"Content-Type": "application/json",
	}

	body := map[string]any{
		"tercero_id": terceroID,
	}

	fmt.Printf("[MID->CRUD] PUT %s\n", endpoint)
	fmt.Printf("[MID->CRUD] headers=%v\n", headers)
	b, _ := json.Marshal(body)
	fmt.Printf("[MID->CRUD] body=%s\n", string(b))

	var updated map[string]interface{}
	if err := helpers.DoJSONWithHeaders(
		"PUT",
		endpoint,
		headers,
		body,
		&updated,
		cfg.RequestTimeout,
		true,
	); err != nil {
		return nil, helpers.AsAppError(err, "error aceptando invitación")
	}

	out := normalizeInvitacion(updated)
	enrichInvitacionEstados([]map[string]interface{}{out})
	attachOfertaResumen([]map[string]interface{}{out})

	// (Opcional)
	if ofertaID := extractOfertaID(out); ofertaID > 0 {
		_ = crearPostulacionDesdeInvitacion(int64(terceroID), ofertaID)
	}

	return out, nil
}

// RechazarInvitacion marca la invitación como rechazada.
func RechazarInvitacion(ctx context.Context, invitacionID int, terceroID int) (map[string]interface{}, error) {
	if invitacionID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "id inválido", nil)
	}
	if terceroID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "tercero_id inválido", nil)
	}

	inv, err := findInvitacionInBandejaEstudiante(ctx, terceroID, invitacionID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "invitación no encontrada", nil)
	}

	tutorID, _ := toInt(inv["tutor_id"])
	if tutorID <= 0 {
		return nil, helpers.NewAppError(http.StatusInternalServerError, "no fue posible determinar tutor_id de la invitación", nil)
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, invitacionesResource, strconv.Itoa(invitacionID), "rechazar")

	var updated map[string]interface{}
	if err := helpers.DoJSONWithHeaders(
		"PUT",
		endpoint,
		map[string]string{"X-Tutor-Id": fmt.Sprint(tutorID)},
		nil,
		&updated,
		cfg.RequestTimeout,
		true,
	); err != nil {
		return nil, helpers.AsAppError(err, "error rechazando invitación")
	}

	out := normalizeInvitacion(updated)
	enrichInvitacionEstados([]map[string]interface{}{out})
	attachOfertaResumen([]map[string]interface{}{out})
	return out, nil
}

// Busca la invitación (por id) dentro de la bandeja del estudiante (CRUD).
func findInvitacionInBandejaEstudiante(ctx context.Context, terceroID int, invitacionID int) (map[string]interface{}, error) {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, invitacionesResource)

	values := url.Values{}
	values.Set("estudiante_id", fmt.Sprint(terceroID))
	values.Set("limit", "0")

	urlWithQuery := endpoint + "?" + values.Encode()

	var raw []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &raw, cfg.RequestTimeout); err != nil {
		return nil, helpers.AsAppError(err, "error consultando invitaciones del estudiante")
	}

	for _, it := range raw {
		n := normalizeInvitacion(it)
		id, _ := toInt(n["id"])
		if id == invitacionID {
			return n, nil
		}
	}
	return nil, nil
}

// helper: convierte varios tipos a int
func toInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n), true
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n, true
		}
	}
	return 0, false
}

// --------- helpers ---------

func normalizeInvitacion(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	out := map[string]interface{}{}

	// id
	if v, ok := data["Id"]; ok {
		out["id"] = v
	} else if v, ok := data["id"]; ok {
		out["id"] = v
	}

	// estado
	if v, ok := data["Estado"]; ok {
		out["estado"] = v
	} else if v, ok := data["estado"]; ok {
		out["estado"] = v
	}

	// mensaje
	if v, ok := data["Mensaje"]; ok {
		out["mensaje"] = v
	} else if v, ok := data["mensaje"]; ok {
		out["mensaje"] = v
	}

	// tutor_id
	if v, ok := data["TutorId"]; ok {
		out["tutor_id"] = v
	} else if v, ok := data["tutor_id"]; ok {
		out["tutor_id"] = v
	}

	// perfil
	if v, ok := data["PerfilId"]; ok {
		out["perfil_estudiante_id"] = v
	} else if v, ok := data["PerfilEstudianteId"]; ok {
		out["perfil_estudiante_id"] = v
	} else if v, ok := data["perfil_estudiante_id"]; ok {
		out["perfil_estudiante_id"] = v
	}

	// oferta (normalizamos a oferta_pasantia_id)
	if v, ok := data["OfertaPasantiaId"]; ok {
		out["oferta_pasantia_id"] = v
	} else if v, ok := data["OfertaId"]; ok {
		out["oferta_pasantia_id"] = v
	} else if v, ok := data["oferta_pasantia_id"]; ok {
		out["oferta_pasantia_id"] = v
	} else if v, ok := data["oferta_id"]; ok {
		out["oferta_pasantia_id"] = v
	}

	// fechas
	if v, ok := data["FechaCreacion"]; ok {
		out["fecha_creacion"] = v
	} else if v, ok := data["fecha_creacion"]; ok {
		out["fecha_creacion"] = v
	}
	if v, ok := data["FechaEstado"]; ok {
		out["fecha_estado"] = v
	} else if v, ok := data["fecha_estado"]; ok {
		out["fecha_estado"] = v
	}

	return out
}

func enrichInvitacionEstados(items []map[string]interface{}) {
	for _, item := range items {
		raw := strings.ToUpper(strings.TrimSpace(fmt.Sprint(item["estado"])))
		code := mapInvEstadoToParamCode(raw)

		nombre := code
		if code != "" {
			if par, err := internalhelpers.GetParametroByCodeNoCache(code); err == nil {
				if n := strings.TrimSpace(par.Nombre); n != "" {
					nombre = n
				}
			}
		}

		item["estado_det"] = map[string]string{
			"code":   code,
			"nombre": nombre,
		}
		item["estado_raw"] = raw
	}
}

func attachOfertaResumen(items []map[string]interface{}) {
	ids := make(map[int64]struct{})
	for _, item := range items {
		if id := extractOfertaID(item); id > 0 {
			ids[id] = struct{}{}
		}
	}

	summaries := make(map[int64]map[string]interface{}, len(ids))
	for id := range ids {
		if oferta, err := rootservices.GetOferta(id); err == nil && oferta != nil {
			summaries[id] = map[string]interface{}{
				"id":     oferta.Id,
				"titulo": strings.TrimSpace(oferta.Titulo),
			}
		}
	}

	for _, item := range items {
		id := extractOfertaID(item)
		if id > 0 {
			item["oferta_pasantia_id"] = id
		}
		if summary, ok := summaries[id]; ok {
			item["oferta_resumen"] = summary
		}
	}
}

func extractOfertaID(item map[string]interface{}) int64 {
	candidates := []string{"oferta_pasantia_id", "ofertapasantiaid", "oferta_id", "ofertaid"}
	for _, key := range candidates {
		if val, ok := item[key]; ok {
			switch t := val.(type) {
			case float64:
				return int64(t)
			case int:
				return int64(t)
			case int64:
				return t
			case json.Number:
				if parsed, err := t.Int64(); err == nil {
					return parsed
				}
			case string:
				if parsed, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
					return parsed
				}
			}
		}
	}
	return 0
}

// Crear postulación desde invitación (si tu flujo lo usa)
func crearPostulacionDesdeInvitacion(terceroID int64, ofertaID int64) error {
	dto := rootmodels.CreatePostulacionDTO{
		EstudianteId: terceroID,
		OfertaId:     ofertaID,
	}
	_, _, err := rootservices.CreatePostulacion(dto)
	return err
}

func enrichInvitacionDetalle(ctx context.Context, inv map[string]interface{}) {
	if inv == nil {
		return
	}

	//marca para verificar que este código sí está corriendo
	inv["_enriched"] = true
	inv["_enriched_at"] = time.Now().Format(time.RFC3339)

	// 0) Estudiante detalle (perfil)
	if perfilID, ok := toInt(inv["perfil_estudiante_id"]); ok && perfilID > 0 {
		crud := clients.CastorCRUD()
		perfil, err := crud.GetPerfilByID(ctx, perfilID)
		if err != nil || perfil == nil {
			inv["estudiante_detalle"] = map[string]any{
				"perfil_id": perfilID,
			}
		} else {
			estudiante := map[string]any{
				"perfil_id": perfilID,
				"visible":   perfil.Visible,
			}
			if perfil.TerceroId > 0 {
				estudiante["tercero_id"] = perfil.TerceroId
				if nombre := strings.TrimSpace(NombreCompletoPorIDCoreStd(ctx, perfil.TerceroId)); nombre != "" {
					estudiante["nombre_completo"] = nombre
				}
			}
			if perfil.ProyectoCurricularId > 0 {
				estudiante["proyecto_curricular_id"] = perfil.ProyectoCurricularId
				if nombre := strings.TrimSpace(nombreProyectoCurricular(ctx, perfil.ProyectoCurricularId)); nombre != "" {
					estudiante["proyecto_curricular_nombre"] = nombre
				}
			}
			if resumen := strings.TrimSpace(perfil.Resumen); resumen != "" {
				estudiante["resumen"] = resumen
			}
			if hab := strings.TrimSpace(perfil.Habilidades); hab != "" {
				estudiante["habilidades"] = hab
			}

			if raw := strings.TrimSpace(string(perfil.CvDocumentoRaw)); raw != "" && raw != "null" {
				var cv string
				if err := json.Unmarshal([]byte(raw), &cv); err == nil && strings.TrimSpace(cv) != "" {
					estudiante["cv_documento_id"] = strings.TrimSpace(cv)
				} else {
					var cvNum int
					if err := json.Unmarshal([]byte(raw), &cvNum); err == nil && cvNum > 0 {
						estudiante["cv_documento_id"] = strconv.Itoa(cvNum)
					} else {
						estudiante["cv_documento_id"] = raw
					}
				}
			}

			inv["estudiante_detalle"] = estudiante
		}
	}

	// 1) Oferta
	ofertaID := extractOfertaID(inv)
	if ofertaID <= 0 {
		return
	}

	oferta, err := rootservices.GetOferta(ofertaID)
	if err != nil || oferta == nil {
		inv["_enrich_oferta_error"] = fmt.Sprint(err)
		return
	}

	// estado_det oferta
	estadoCode := strings.TrimSpace(oferta.Estado)
	estadoNombre := estadoCode
	if estadoCode != "" {
		if par, e := internalhelpers.GetParametroByCodeNoCache(estadoCode); e == nil {
			if n := strings.TrimSpace(par.Nombre); n != "" {
				estadoNombre = n
			}
		}
	}
	ofertaEstadoDet := map[string]any{
		"code":   estadoCode,
		"nombre": estadoNombre,
	}

	// oferta_detalle (lo mínimo necesario para la UI del detalle)
	inv["oferta_detalle"] = map[string]any{
		"id":                      oferta.Id,
		"titulo":                  strings.TrimSpace(oferta.Titulo),
		"descripcion":             strings.TrimSpace(oferta.Descripcion),
		"estado":                  estadoCode,
		"estado_det":              ofertaEstadoDet,
		"empresa_id":              oferta.EmpresaId,
		"tutor_externo_id":        oferta.TutorExternoId,
		"fecha_publicacion":       oferta.FechaPublicacion,
		"proyecto_curricular_ids": oferta.ProyectoCurricularIds,
	}

	// 2) Empresa detalle (Terceros)
	empresaID := int(oferta.EmpresaId)
	if empresaID > 0 {
		emp, e := ObtenerTerceroPorIDCoreStd(ctx, empresaID)
		if e == nil && emp != nil {
			inv["empresa_detalle"] = map[string]any{
				"tercero_id":      empresaID,
				"nombre_completo": strings.TrimSpace(fmt.Sprint(emp["NombreCompleto"])),
				// "raw": emp,
			}
		} else {
			inv["_enrich_empresa_error"] = fmt.Sprint(e)
			inv["empresa_detalle"] = map[string]any{
				"tercero_id": empresaID,
			}
		}
	}

	// 3) Tutor detalle (Terceros)
	tutorID, _ := toInt(inv["tutor_id"])
	if tutorID <= 0 {
		tutorID = int(oferta.TutorExternoId)
	}
	if tutorID > 0 {
		tut, e := ObtenerTerceroPorIDCoreStd(ctx, tutorID)
		if e == nil && tut != nil {
			inv["tutor_detalle"] = map[string]any{
				"tercero_id":      tutorID,
				"nombre_completo": strings.TrimSpace(fmt.Sprint(tut["NombreCompleto"])),
				// "raw": tut,
			}
		} else {
			inv["_enrich_tutor_error"] = fmt.Sprint(e)
			inv["tutor_detalle"] = map[string]any{
				"tercero_id": tutorID,
			}
		}
	}
}

func enrichInvitacionConEstudianteDetalle(ctx context.Context, inv map[string]interface{}) {
	if inv == nil {
		return
	}

	perfilID, ok := toInt(inv["perfil_estudiante_id"])
	if !ok || perfilID <= 0 {
		return
	}

	crud := clients.CastorCRUD()
	perfilRec, err := crud.GetPerfilByID(ctx, perfilID)
	if err != nil || perfilRec == nil {

		return
	}

	terceroID := perfilRec.TerceroId
	nombre := ""
	if terceroID > 0 {
		nombre = strings.TrimSpace(NombreCompletoPorIDCoreStd(ctx, terceroID))
	}

	detalle, _ := inv["estudiante_detalle"].(map[string]any)
	if detalle == nil {
		detalle = map[string]any{}
	}

	detalle["perfil_id"] = perfilID
	if terceroID > 0 {
		detalle["tercero_id"] = terceroID
	}
	if nombre != "" {
		detalle["nombre_completo"] = nombre
	}
	if perfilRec.ProyectoCurricularId > 0 {
		detalle["proyecto_curricular_id"] = perfilRec.ProyectoCurricularId
	}

	inv["estudiante_detalle"] = detalle
}

func enrichInvitacionesConEstudianteResumen(ctx context.Context, items []map[string]interface{}) {
	if len(items) == 0 {
		return
	}

	perfilIDs := make(map[int]struct{})
	for _, item := range items {
		if pid, ok := toInt(item["perfil_estudiante_id"]); ok && pid > 0 {
			perfilIDs[pid] = struct{}{}
		}
	}
	if len(perfilIDs) == 0 {
		return
	}

	crud := clients.CastorCRUD()
	perfilCache := make(map[int]*clients.PerfilRecord, len(perfilIDs))
	terceroIDs := make(map[int]struct{})
	pcIDs := make(map[int]struct{})

	for pid := range perfilIDs {
		perfil, err := crud.GetPerfilByID(ctx, pid)
		if err != nil || perfil == nil {
			continue
		}
		perfilCache[pid] = perfil
		if perfil.TerceroId > 0 {
			terceroIDs[perfil.TerceroId] = struct{}{}
		}
		if perfil.ProyectoCurricularId > 0 {
			pcIDs[perfil.ProyectoCurricularId] = struct{}{}
		}
	}

	nombreCache := make(map[int]string, len(terceroIDs))
	for tid := range terceroIDs {
		if tid <= 0 {
			continue
		}
		nombre := strings.TrimSpace(NombreCompletoPorIDCoreStd(ctx, tid))
		if nombre != "" {
			nombreCache[tid] = nombre
		}
	}

	pcNombreCache := make(map[int]string, len(pcIDs))
	for pcID := range pcIDs {
		if pcID <= 0 {
			continue
		}
		if nombre := strings.TrimSpace(nombreProyectoCurricular(ctx, pcID)); nombre != "" {
			pcNombreCache[pcID] = nombre
		}
	}

	for _, item := range items {
		pid, ok := toInt(item["perfil_estudiante_id"])
		if !ok || pid <= 0 {
			continue
		}
		resumen := map[string]any{
			"perfil_id": pid,
		}
		if perfil := perfilCache[pid]; perfil != nil {
			if perfil.TerceroId > 0 {
				resumen["tercero_id"] = perfil.TerceroId
				if nombre, ok := nombreCache[perfil.TerceroId]; ok && strings.TrimSpace(nombre) != "" {
					resumen["nombre_completo"] = nombre
				}
			}
			if perfil.ProyectoCurricularId > 0 {
				resumen["proyecto_curricular_id"] = perfil.ProyectoCurricularId
				if nombre, ok := pcNombreCache[perfil.ProyectoCurricularId]; ok && strings.TrimSpace(nombre) != "" {
					resumen["proyecto_curricular_nombre"] = nombre
				}
			}
		}
		item["estudiante_resumen"] = resumen
	}
}

func nombreProyectoCurricular(ctx context.Context, pcID int) string {
	if pcID <= 0 {
		return ""
	}
	detalle, err := rootservices.GetProyectoCurricular(pcID)
	if err != nil || detalle == nil {
		return ""
	}
	return strings.TrimSpace(detalle.Nombre)
}

func enrichBandejaTutorConEstudiantes(ctx context.Context, items []map[string]interface{}) {
	if len(items) == 0 {
		return
	}

	perfilIDs := make(map[int]struct{})
	for _, item := range items {
		if pid, ok := toInt(item["perfil_estudiante_id"]); ok && pid > 0 {
			perfilIDs[pid] = struct{}{}
		}
	}
	if len(perfilIDs) == 0 {
		return
	}

	crud := clients.CastorCRUD()
	perfilCache := make(map[int]*clients.PerfilRecord, len(perfilIDs))
	for pid := range perfilIDs {
		perfil, err := crud.GetPerfilByID(ctx, pid)
		if err != nil || perfil == nil {
			continue
		}
		perfilCache[pid] = perfil
	}

	nombreCache := make(map[int]string)

	for _, item := range items {
		pid, ok := toInt(item["perfil_estudiante_id"])
		if !ok || pid <= 0 {
			continue
		}
		perfil := perfilCache[pid]
		if perfil == nil {
			continue
		}

		resumen := map[string]any{
			"perfil_id": pid,
		}

		terceroID := perfil.TerceroId
		if terceroID > 0 {
			resumen["tercero_id"] = terceroID
			if nombre, ok := nombreCache[terceroID]; ok {
				if strings.TrimSpace(nombre) != "" {
					resumen["nombre_completo"] = nombre
				}
			} else {
				nombre := strings.TrimSpace(NombreCompletoPorIDCoreStd(ctx, terceroID))
				nombreCache[terceroID] = nombre
				if nombre != "" {
					resumen["nombre_completo"] = nombre
				}
			}
		}

		if perfil.ProyectoCurricularId > 0 {
			resumen["proyecto_curricular_id"] = perfil.ProyectoCurricularId
		}

		if perfil.Extra != nil {
			if v, ok := perfil.Extra["ProyectoCurricularNombre"]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					resumen["proyecto_curricular_nombre"] = strings.TrimSpace(s)
				}
			} else if v, ok := perfil.Extra["proyecto_curricular_nombre"]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					resumen["proyecto_curricular_nombre"] = strings.TrimSpace(s)
				}
			}
		}

		item["estudiante_resumen"] = resumen
	}
}
