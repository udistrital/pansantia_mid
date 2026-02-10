package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/udistrital/pasantia_mid/helpers"
	internaldto "github.com/udistrital/pasantia_mid/internal/dto"
	rootservices "github.com/udistrital/pasantia_mid/services"

	"github.com/beego/beego/v2/server/web/context"
)

const (
	bookmarkResource = "tutor_bookmark"
	visitaResource   = "perfil_visita"
	perfilResource   = "estudiante_perfil"
	defaultOrder     = "FechaModificacion"
)

// CatalogoFilters agrupa los filtros disponibles para el catálogo.
type CatalogoFilters struct {
	ProyectoCurricularID *int
	Skills               string
	Query                string
	HabilidadesCSV       string
	Page                 int
	Size                 int
}

// Catalogo retorna la página solicitada del catálogo de estudiantes.
func Catalogo(ctx *context.Context, filters CatalogoFilters, tutorID int) (internaldto.PageDTO[internaldto.EstudiantePerfilCard], error) {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, "explorar", "estudiantes")
	values := url.Values{}

	if filters.ProyectoCurricularID != nil && *filters.ProyectoCurricularID > 0 {
		values.Set("pc_id", strconv.Itoa(*filters.ProyectoCurricularID))
	}
	if trimmed := strings.TrimSpace(filters.Skills); trimmed != "" {
		values.Set("skills", trimmed)
	}
	if trimmed := strings.TrimSpace(filters.Query); trimmed != "" {
		values.Set("q", trimmed)
	}
	if filters.Page > 0 {
		values.Set("page", strconv.Itoa(filters.Page))
	}
	if filters.Size > 0 {
		values.Set("size", strconv.Itoa(filters.Size))
	}
	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var raw struct {
		Items []map[string]interface{} `json:"items"`
		Page  int                      `json:"page"`
		Size  int                      `json:"size"`
		Total int64                    `json:"total"`
	}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &raw, cfg.RequestTimeout); err != nil {
		return internaldto.PageDTO[internaldto.EstudiantePerfilCard]{}, helpers.AsAppError(err, "error consultando catálogo")
	}

	pcNames := map[int64]string{}
	pcIDs := collectPCIDs(raw.Items)
	for _, pcID := range pcIDs {
		if detalle, err := rootservices.GetProyectoCurricular(int(pcID)); err == nil && detalle != nil {
			pcNames[pcID] = strings.TrimSpace(detalle.Nombre)
		}
	}

	return internaldto.PageDTO[internaldto.EstudiantePerfilCard]{
		Items: mapToPerfilCards(raw.Items, pcNames),
		Page:  raw.Page,
		Size:  raw.Size,
		Total: raw.Total,
	}, nil
}

// DetallePerfil obtiene el detalle del perfil, marcando si está guardado por el tutor.
func DetallePerfil(ctx *context.Context, perfilID int, tutorID int) (map[string]interface{}, error) {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, perfilResource, strconv.Itoa(perfilID))

	var record map[string]interface{}
	if err := helpers.DoJSON("GET", endpoint, nil, &record, cfg.RequestTimeout); err != nil {
		if helpers.IsHTTPError(err, http.StatusNotFound) {
			return nil, helpers.NewAppError(http.StatusNotFound, "perfil no encontrado", nil)
		}
		return nil, helpers.AsAppError(err, "error consultando perfil")
	}

	perfil := map[string]interface{}{}
	for k, v := range record {
		perfil[strings.ToLower(k)] = v
	}

	if val, ok := record["TratamientoDatosAceptado"]; ok {
		perfil["tratamiento_datos_aceptado"] = normalizeToBool(val, false)
	} else if val, ok := perfil["tratamientodatosaceptado"]; ok {
		perfil["tratamiento_datos_aceptado"] = normalizeToBool(val, false)
	}

	_ = tutorID

	if pcRaw, ok := perfil["proyectocurricularid"]; ok {
		if pcID, ok := normalizeToInt(pcRaw); ok && pcID > 0 {
			if detalle, err := rootservices.GetProyectoCurricular(pcID); err == nil && detalle != nil {
				perfil["proyecto_curricular"] = map[string]interface{}{
					"id":     detalle.Id,
					"nombre": strings.TrimSpace(detalle.Nombre),
				}
			}
		}
	}

	return perfil, nil
}

// GuardarPerfil registra un bookmark tutor-perfil.
func GuardarPerfil(ctx *context.Context, tutorID, perfilID int) error {
	if bookmarkExists(tutorID, perfilID) {
		return nil
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, bookmarkResource)
	body := map[string]interface{}{
		"TutorId":            tutorID,
		"PerfilEstudianteId": perfilID,
		"FechaCreacion":      nowISO(),
		"FechaModificacion":  nowISO(),
	}

	var created map[string]interface{}
	if err := helpers.DoJSON("POST", endpoint, body, &created, cfg.RequestTimeout); err != nil {
		return helpers.AsAppError(err, "error guardando perfil")
	}
	return nil
}

// EliminarBookmark elimina la relación tutor-perfil si existe.
func EliminarBookmark(ctx *context.Context, tutorID, perfilID int) error {
	id, err := findBookmarkID(tutorID, perfilID)
	if err != nil {
		return err
	}
	if id == 0 {
		return helpers.NewAppError(http.StatusNotFound, "bookmark no encontrado", nil)
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, bookmarkResource, strconv.Itoa(id))

	if err := helpers.DoJSON("DELETE", endpoint, nil, nil, cfg.RequestTimeout); err != nil {
		return helpers.AsAppError(err, "error eliminando bookmark")
	}
	return nil
}

// RegistrarVisita crea un registro de visita tutor-perfil.
func RegistrarVisita(ctx *context.Context, tutorID, perfilID int) error {
	cfg := rootservices.GetConfig()

	//Endpoint REAL del CRUD según tu router.go
	endpoint := rootservices.BuildURL(
		cfg.CastorCRUDBaseURL,
		"explorar", "estudiantes", strconv.Itoa(perfilID), "visita",
	)

	// El CRUD espera tutor por header
	headers := map[string]string{"X-Tutor-Id": fmt.Sprint(tutorID)}

	var created map[string]interface{}
	if err := helpers.DoJSONWithHeaders(
		"POST",
		endpoint,
		headers,
		nil, // body vacío
		&created,
		cfg.RequestTimeout,
		true, // porque BaseController.Created / Ok suele venir envuelto
	); err != nil {
		return helpers.AsAppError(err, "error registrando visita")
	}

	return nil
}

func buildCatalogQuery(filters CatalogoFilters) url.Values {
	values := url.Values{}
	queryParts := []string{"Visible:true"}

	if filters.ProyectoCurricularID != nil && *filters.ProyectoCurricularID > 0 {
		queryParts = append(queryParts, fmt.Sprintf("ProyectoCurricularId:%d", *filters.ProyectoCurricularID))
	}
	if trimmed := strings.TrimSpace(filters.Skills); trimmed != "" {
		queryParts = append(queryParts, fmt.Sprintf("Habilidades__icontains:%s", trimmed))
	}
	if trimmed := strings.TrimSpace(filters.HabilidadesCSV); trimmed != "" {
		parts := strings.Split(trimmed, ",")
		var clauses []string
		seen := make(map[string]struct{})
		for _, part := range parts {
			s := strings.TrimSpace(part)
			if s == "" {
				continue
			}
			key := strings.ToLower(s)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			clauses = append(clauses, fmt.Sprintf("Habilidades__icontains:%s", s))
		}
		if len(clauses) == 1 {
			queryParts = append(queryParts, clauses[0])
		} else if len(clauses) > 1 {
			queryParts = append(queryParts, fmt.Sprintf("(%s)", strings.Join(clauses, "|")))
		}
	}
	if trimmed := strings.TrimSpace(filters.Query); trimmed != "" {
		queryParts = append(queryParts, fmt.Sprintf("Resumen__icontains:%s", trimmed))
	}

	values.Set("query", strings.Join(queryParts, ","))
	values.Set("sortby", defaultOrder)
	values.Set("order", "desc")
	return values
}

func bookmarkSetForTutor(tutorID int) map[int]struct{} {
	set := make(map[int]struct{})
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, bookmarkResource)
	values := url.Values{}
	values.Set("limit", "0")
	values.Set("query", fmt.Sprintf("TutorId:%d", tutorID))

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var bookmarks []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &bookmarks, cfg.RequestTimeout); err != nil {
		return set
	}
	for _, entry := range bookmarks {
		if perfilID, ok := normalizeToInt(entry["PerfilEstudianteId"]); ok {
			set[perfilID] = struct{}{}
		}
	}
	return set
}

func bookmarkExists(tutorID, perfilID int) bool {
	id, err := findBookmarkID(tutorID, perfilID)
	return err == nil && id > 0
}

func findBookmarkID(tutorID, perfilID int) (int, error) {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, bookmarkResource)

	values := url.Values{}
	values.Set("limit", "1")
	values.Set("query", fmt.Sprintf("TutorId:%d,PerfilEstudianteId:%d", tutorID, perfilID))

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var records []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &records, cfg.RequestTimeout); err != nil {
		if helpers.IsHTTPError(err, http.StatusNotFound) {
			return 0, nil
		}
		return 0, helpers.AsAppError(err, "error consultando bookmark")
	}
	if len(records) == 0 {
		return 0, nil
	}
	if id, ok := normalizeToInt(records[0]["Id"]); ok {
		return id, nil
	}
	return 0, nil
}

func mapToPerfilCard(entry map[string]interface{}) internaldto.EstudiantePerfilCard {
	perfilID, _ := normalizeToInt64(entry["Id"])
	terceroID, _ := normalizeToInt64(entry["TerceroId"])
	pcID, _ := normalizeToInt64(entry["ProyectoCurricularId"])

	card := internaldto.EstudiantePerfilCard{
		PerfilID:                 perfilID,
		TerceroID:                terceroID,
		ProyectoCurricularID:     pcID,
		Resumen:                  strings.TrimSpace(normalizeToString(entry["Resumen"])),
		Habilidades:              parseHabilidades(entry["Habilidades"]),
		Visible:                  normalizeToBool(entry["Visible"], true),
		Guardado:                 normalizeToBool(entry["Guardado"], false),
		TratamientoDatosAceptado: normalizeToBool(entry["TratamientoDatosAceptado"], false),
	}

	if docID, ok := normalizeToInt(entry["CvDocumentoId"]); ok {
		val := strconv.Itoa(docID)
		card.CVDocumentoID = &val
	}
	return card
}

func mapToPerfilCards(items []map[string]interface{}, pcNames map[int64]string) []internaldto.EstudiantePerfilCard {
	out := make([]internaldto.EstudiantePerfilCard, 0, len(items))
	for _, entry := range items {
		out = append(out, mapToPerfilCardExplorar(entry, pcNames))
	}
	return out
}

func mapToPerfilCardExplorar(entry map[string]interface{}, pcNames map[int64]string) internaldto.EstudiantePerfilCard {
	perfilID, _ := normalizeToInt64(entry["Id"])
	if perfilID == 0 {
		perfilID, _ = normalizeToInt64(entry["id"])
	}
	terceroID, _ := normalizeToInt64(entry["TerceroId"])
	if terceroID == 0 {
		terceroID, _ = normalizeToInt64(entry["tercero_id"])
	}
	pcID, _ := normalizeToInt64(entry["ProyectoCurricularId"])
	if pcID == 0 {
		pcID, _ = normalizeToInt64(entry["proyecto_curricular_id"])
	}

	card := internaldto.EstudiantePerfilCard{
		PerfilID:                 perfilID,
		TerceroID:                terceroID,
		ProyectoCurricularID:     pcID,
		Resumen:                  strings.TrimSpace(normalizeToString(entry["Resumen"])),
		Habilidades:              parseHabilidades(entry["Habilidades"]),
		Visible:                  normalizeToBool(entry["Visible"], true),
		Guardado:                 false,
		TratamientoDatosAceptado: normalizeToBool(entry["TratamientoDatosAceptado"], false),
	}
	if card.Resumen == "" {
		card.Resumen = strings.TrimSpace(normalizeToString(entry["resumen"]))
	}
	if len(card.Habilidades) == 0 {
		card.Habilidades = parseHabilidades(entry["habilidades"])
	}
	if !card.Visible {
		card.Visible = normalizeToBool(entry["visible"], card.Visible)
	}
	if !card.TratamientoDatosAceptado {
		card.TratamientoDatosAceptado = normalizeToBool(entry["tratamiento_datos_aceptado"], card.TratamientoDatosAceptado)
	}

	if docID, ok := normalizeToInt(entry["CvDocumentoId"]); ok {
		val := strconv.Itoa(docID)
		card.CVDocumentoID = &val
	} else if docID, ok := normalizeToInt(entry["cv_documento_id"]); ok {
		val := strconv.Itoa(docID)
		card.CVDocumentoID = &val
	}
	if pcID > 0 {
		if nombre, ok := pcNames[pcID]; ok && strings.TrimSpace(nombre) != "" {
			card.ProyectoCurricularNombre = strings.TrimSpace(nombre)
			card.ProyectoCurricular = map[string]interface{}{
				"id":     pcID,
				"nombre": strings.TrimSpace(nombre),
			}
		}
	}
	return card
}

func collectPCIDs(items []map[string]interface{}) []int64 {
	seen := map[int64]struct{}{}
	for _, entry := range items {
		pcID, _ := normalizeToInt64(entry["ProyectoCurricularId"])
		if pcID == 0 {
			pcID, _ = normalizeToInt64(entry["proyecto_curricular_id"])
		}
		if pcID > 0 {
			seen[pcID] = struct{}{}
		}
	}
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out
}

func parseHabilidades(value interface{}) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var arr []string
		if json.Unmarshal([]byte(v), &arr) == nil {
			return arr
		}
		return []string{strings.TrimSpace(v)}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(normalizeToString(item)); s != "" {
				result = append(result, s)
			}
		}
		return result
	case []string:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	default:
		return nil
	}
}
