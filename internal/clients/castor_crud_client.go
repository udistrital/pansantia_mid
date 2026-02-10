package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/models"
	rootservices "github.com/udistrital/pasantia_mid/services"
)

// CastorCRUDClient wraps the operations against the castor_crud service that are required by the MID.
type CastorCRUDClient struct {
	cfg rootservices.Config
}

type EstudiantePerfil struct {
	Id            int
	TerceroId     int
	Visible       bool
	CvDocumentoId string
}

type CastorCrudClient struct{}

var (
	castorClient     *CastorCRUDClient
	castorClientOnce sync.Once
)

// CastorCRUD returns a singleton client ready to call the CRUD service.
func CastorCRUD() *CastorCRUDClient {
	castorClientOnce.Do(func() {
		castorClient = &CastorCRUDClient{
			cfg: rootservices.GetConfig(),
		}
	})
	return castorClient
}

// GetPostulacionByID fetches a postulación record by its identifier.
func (c *CastorCRUDClient) GetPostulacionByID(ctx context.Context, id int64) (*models.Postulacion, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	endpoint := rootservices.BuildURL(c.cfg.CastorCRUDBaseURL, "postulacion", strconv.FormatInt(id, 10))

	var raw postulacionRecord
	if err := helpers.DoJSON("GET", endpoint, nil, &raw, c.cfg.RequestTimeout); err != nil {
		return nil, err
	}
	post := mapPostulacion(raw)
	return &post, nil
}

// UpdatePostulacionEstado updates the state of a postulación.
func (c *CastorCRUDClient) UpdatePostulacionEstado(ctx context.Context, id int64, estado string, when time.Time) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	endpoint := rootservices.BuildURL(c.cfg.CastorCRUDBaseURL, "postulacion", strconv.FormatInt(id, 10))

	var current map[string]any
	if err := helpers.DoJSON("GET", endpoint, nil, &current, c.cfg.RequestTimeout); err != nil {
		fmt.Println("UpdatePostulacionEstado GET error:", "id", id, "estado", estado, "err", err)
		return err
	}
	if current == nil || len(current) == 0 {
		return helpers.NewAppError(http.StatusNotFound, "postulación no encontrada", nil)
	}
	if _, ok := current["OfertaPasantiaId"]; !ok || current["OfertaPasantiaId"] == nil {
		return helpers.NewAppError(http.StatusBadRequest, "postulación sin OfertaPasantiaId", nil)
	}

	normalized := strings.ToUpper(strings.TrimSpace(estado))
	if normalized == "" {
		return helpers.NewAppError(http.StatusBadRequest, "estado requerido", nil)
	}
	current["estado_postulacion"] = normalized

	now := time.Now().UTC()
	if !when.IsZero() {
		now = when.UTC()
	}
	current["fecha_estado"] = now.Format(time.RFC3339)

	var updated map[string]any
	if err := helpers.DoJSON("PUT", endpoint, current, &updated, c.cfg.RequestTimeout); err != nil {
		fmt.Println("UpdatePostulacionEstado PUT error:", "id", id, "estado", normalized, "err", err)
		return err
	}
	return nil
}

// AddPostulacionRevision registers an action on a postulación.
func (c *CastorCRUDClient) AddPostulacionRevision(ctx context.Context, postulacionID int64, tutorID int, accion, comentario string, when time.Time) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	endpoint := rootservices.BuildURL(c.cfg.CastorCRUDBaseURL, "postulacion_revision")
	body := map[string]interface{}{
		"TutorId":       tutorID,
		"PostulacionId": postulacionID,
		"Accion":        strings.TrimSpace(accion),
		"Fecha":         when.UTC().Format(time.RFC3339),
	}
	if note := strings.TrimSpace(comentario); note != "" {
		body["Comentario"] = note
	}

	var created map[string]interface{}
	return helpers.DoJSON("POST", endpoint, body, &created, c.cfg.RequestTimeout)
}

// ListPostulaciones retrieves postulation records applying CRUD filters.
func (c *CastorCRUDClient) ListPostulaciones(ctx context.Context, filters map[string]string) ([]models.Postulacion, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	endpoint := rootservices.BuildURL(c.cfg.CastorCRUDBaseURL, "postulacion")
	values := buildPostulacionFilters(filters)
	if encoded := values.Encode(); encoded != "" {
		endpoint = endpoint + "?" + encoded
	}

	var raw []postulacionRecord
	if err := helpers.DoJSON("GET", endpoint, nil, &raw, c.cfg.RequestTimeout); err != nil {
		return nil, err
	}

	result := make([]models.Postulacion, 0, len(raw))
	for _, item := range raw {
		result = append(result, mapPostulacion(item))
	}
	return result, nil
}

// GetPerfilByTerceroID returns the profile associated with the given tercero (estudiante).
func (c *CastorCRUDClient) GetPerfilByTerceroID(ctx context.Context, terceroID int) (*PerfilRecord, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	endpoint := rootservices.BuildURL(c.cfg.CastorCRUDBaseURL, "estudiante_perfil")
	values := url.Values{}
	values.Set("limit", "1")
	values.Set("query", fmt.Sprintf("TerceroId:%d", terceroID))

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var records []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &records, c.cfg.RequestTimeout); err != nil {
		if helpers.IsHTTPError(err, http.StatusNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	rec := mapPerfilRecord(records[0])
	return &rec, nil
}

// GetPerfilByID returns a profile by its identifier.
func (c *CastorCRUDClient) GetPerfilByID(ctx context.Context, perfilID int) (*PerfilRecord, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	if perfilID <= 0 {
		return nil, nil
	}

	endpoint := rootservices.BuildURL(c.cfg.CastorCRUDBaseURL, "estudiante_perfil", fmt.Sprint(perfilID))

	var raw map[string]interface{}
	if err := helpers.DoJSON("GET", endpoint, nil, &raw, c.cfg.RequestTimeout); err != nil {
		return nil, err
	}

	rec := mapPerfilRecord(raw)
	return &rec, nil
}

// ListPerfilVisitas lists visits registered for a student's profile.
// CRUD real: GET /v1/perfil_visita?perfil_id=...
// Respuesta CRUD: { success,status,message,data:{ items:[], total,... } }
// ListPerfilVisitas -> CRUD: GET /v1/perfil_visita?perfil_id=...
// helpers.DoJSON "desenvuelve" wrapper y decodifica el objeto "data" directamente.
func (c *CastorCRUDClient) ListPerfilVisitas(ctx context.Context, perfilID int) ([]PerfilVisita, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	if perfilID <= 0 {
		return []PerfilVisita{}, nil
	}

	endpoint := rootservices.BuildURL(c.cfg.CastorCRUDBaseURL, "perfil_visita")
	values := url.Values{}
	values.Set("perfil_id", fmt.Sprint(perfilID))
	values.Set("limit", "0")

	urlWithQuery := endpoint + "?" + values.Encode()

	//inner data
	var data struct {
		Items  []map[string]interface{} `json:"items"`
		Total  int                      `json:"total"`
		Limit  int                      `json:"limit"`
		Offset int                      `json:"offset"`
	}

	if err := helpers.DoJSON("GET", urlWithQuery, nil, &data, c.cfg.RequestTimeout); err != nil {
		return nil, err
	}

	rawItems := data.Items
	if rawItems == nil {
		rawItems = []map[string]interface{}{}
	}

	result := make([]PerfilVisita, 0, len(rawItems))
	for _, entry := range rawItems {
		result = append(result, mapPerfilVisita(entry))
	}
	return result, nil
}

// ListInvitaciones lists invitations applying the provided filters and pagination.
func (c *CastorCRUDClient) ListInvitaciones(ctx context.Context, filters map[string]string, offset, limit int) ([]map[string]interface{}, int, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, 0, err
	}
	endpoint := rootservices.BuildURL(c.cfg.CastorCRUDBaseURL, "invitaciones")
	values := url.Values{}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	} else {
		values.Set("limit", "0")
	}
	if offset > 0 {
		values.Set("offset", strconv.Itoa(offset))
	}

	var queryParts []string
	for key, value := range filters {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		field := normalizeInvitacionFilterKey(key)
		queryParts = append(queryParts, fmt.Sprintf("%s:%s", field, trimmed))
	}
	if len(queryParts) > 0 {
		values.Set("query", strings.Join(queryParts, ","))
	}

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var raw []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &raw, c.cfg.RequestTimeout); err != nil {
		return nil, 0, err
	}
	return raw, len(raw), nil
}

// CountPostulacionesByEstado returns a map estado->conteo using MID aggregation.
func (c *CastorCRUDClient) CountPostulacionesByEstado(ctx context.Context, filters map[string]string) (map[string]int, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	result := map[string]int{}
	postulaciones, err := c.ListPostulaciones(ctx, filters)
	if err != nil {
		return nil, err
	}
	for _, p := range postulaciones {
		estado := strings.ToUpper(strings.TrimSpace(p.EstadoPostulacion))
		result[estado]++
	}
	return result, nil
}

// CountInvitacionesByEstado counts invitations grouped by estado applying filters.
func (c *CastorCRUDClient) CountInvitacionesByEstado(ctx context.Context, filters map[string]string) (map[string]int, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	result := map[string]int{}
	invitaciones, _, err := c.ListInvitaciones(ctx, filters, 0, 0)
	if err != nil {
		return nil, err
	}
	for _, inv := range invitaciones {
		estado := strings.ToUpper(strings.TrimSpace(normalizeToString(inv["Estado"])))
		result[estado]++
	}
	return result, nil
}

// ResumenVisitas returns total visits and visits from the last 24h for a profile.
func (c *CastorCRUDClient) ResumenVisitas(ctx context.Context, perfilID int) (map[string]int, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	visitas, err := c.ListPerfilVisitas(ctx, perfilID)
	if err != nil {
		return nil, err
	}
	total := len(visitas)
	var hoy int
	now := time.Now().UTC()
	for _, visita := range visitas {
		if visita.FechaVisita.IsZero() {
			continue
		}
		if now.Sub(visita.FechaVisita) <= 24*time.Hour {
			hoy++
		}
	}
	return map[string]int{
		"total": total,
		"hoy":   hoy,
	}, nil
}

// ResumenOfertasTutor agrupa las ofertas del tutor por estado.
func (c *CastorCRUDClient) ResumenOfertasTutor(ctx context.Context, tutorID int) (map[string]int, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	cfg := c.cfg
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia")
	values := url.Values{}
	values.Set("limit", "0")
	values.Set("query", fmt.Sprintf("TutorExternoId:%d", tutorID))

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var raw []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &raw, cfg.RequestTimeout); err != nil {
		return nil, err
	}

	result := map[string]int{}
	for _, entry := range raw {
		estado := strings.ToUpper(strings.TrimSpace(normalizeToString(entry["Estado"])))
		if estado == "" {
			estado = strings.ToUpper(strings.TrimSpace(normalizeToString(entry["EstadoOferta"])))
		}
		if estado == "" {
			estado = strings.ToUpper(strings.TrimSpace(normalizeToString(entry["estado"])))
		}
		result[estado]++
	}
	return result, nil
}

type OfertaTutorMin struct {
	Id     int64  `json:"id"`
	Titulo string `json:"titulo"`
	Estado string `json:"estado"`
}

func (c *CastorCRUDClient) ListOfertasTutor(ctx context.Context, tutorID int) ([]OfertaTutorMin, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	cfg := c.cfg
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia")
	values := url.Values{}
	values.Set("limit", "0")
	values.Set("query", fmt.Sprintf("TutorExternoId:%d", tutorID))

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var raw []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &raw, cfg.RequestTimeout); err != nil {
		return nil, err
	}

	out := make([]OfertaTutorMin, 0, len(raw))
	for _, entry := range raw {
		var item OfertaTutorMin
		if id, ok := normalizeToInt64(entry["Id"]); ok {
			item.Id = id
		} else if id, ok := normalizeToInt64(entry["id"]); ok {
			item.Id = id
		}
		item.Titulo = strings.TrimSpace(normalizeToString(entry["Titulo"]))
		if item.Titulo == "" {
			item.Titulo = strings.TrimSpace(normalizeToString(entry["titulo"]))
		}
		item.Estado = strings.ToUpper(strings.TrimSpace(normalizeToString(entry["Estado"])))
		if item.Estado == "" {
			item.Estado = strings.ToUpper(strings.TrimSpace(normalizeToString(entry["estado"])))
		}
		out = append(out, item)
	}
	return out, nil
}

func buildPostulacionFilters(filters map[string]string) url.Values {
	values := url.Values{}
	if filters == nil {
		filters = map[string]string{}
	}
	if _, ok := filters["limit"]; !ok {
		values.Set("limit", "0")
	}

	var queryParts []string
	for key, value := range filters {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(strings.TrimSpace(key))
		switch lower {
		case "limit", "offset", "fields", "sortby", "order":
			values.Set(key, trimmed)
			continue
		case "query":
			queryParts = append(queryParts, trimmed)
			continue
		}

		field := normalizePostulacionFilterKey(key)
		queryParts = append(queryParts, fmt.Sprintf("%s:%s", field, trimmed))
	}

	if len(queryParts) > 0 {
		values.Set("query", strings.Join(queryParts, ","))
	}
	return values
}

func normalizePostulacionFilterKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if strings.Contains(trimmed, "__") {
		return trimmed
	}

	switch strings.ToLower(trimmed) {
	case "estudiante_id", "estudianteid":
		return "EstudianteId"
	case "oferta_id", "ofertaid":
		return "OfertaPasantiaId.Id"
	case "estado_postulacion":
		return "EstadoPostulacion__iexact"
	case "estadopostulacion":
		return "EstadoPostulacion"
	case "id":
		return "Id"
	default:
		return trimmed
	}
}

func mapPostulacion(raw postulacionRecord) models.Postulacion {
	return models.Postulacion{
		Id:                raw.Id,
		EstudianteId:      raw.EstudianteId,
		OfertaId:          extractOfertaID(raw.OfertaPasantiaId),
		EstadoPostulacion: strings.TrimSpace(raw.EstadoPostulacion),
		FechaPostulacion:  strings.TrimSpace(raw.FechaPostulacion),
		EnlaceDocHv:       strings.TrimSpace(raw.EnlaceDocHv),
	}
}

func extractOfertaID(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var wrapper struct {
		Id interface{} `json:"Id"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil {
		if id, ok := normalizeToInt64(wrapper.Id); ok {
			return id
		}
	}
	var simple interface{}
	if err := json.Unmarshal(raw, &simple); err == nil {
		if id, ok := normalizeToInt64(simple); ok {
			return id
		}
	}
	return 0
}

// ResumenVisitasPerfil devuelve { total, hoy, items } a partir de:
// GET /v1/perfil_visita/resumen?perfil_id=...
// Nota: helpers.DoJSON suele "desenvolver" el wrapper y entregar directamente el objeto "data".
func (c *CastorCRUDClient) ResumenVisitasPerfil(ctx context.Context, perfilID int, top int) (map[string]interface{}, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	if perfilID <= 0 {
		return map[string]interface{}{"total": 0, "hoy": 0, "items": []map[string]interface{}{}}, nil
	}
	if top <= 0 {
		top = 5
	}

	endpoint := rootservices.BuildURL(c.cfg.CastorCRUDBaseURL, "perfil_visita", "resumen")
	values := url.Values{}
	values.Set("perfil_id", fmt.Sprint(perfilID))
	urlWithQuery := endpoint + "?" + values.Encode()

	// OJO: aquí mapea el "inner data" (no wrapper)
	var data struct {
		Items []map[string]interface{} `json:"items"`
	}

	if err := helpers.DoJSON("GET", urlWithQuery, nil, &data, c.cfg.RequestTimeout); err != nil {
		return nil, err
	}

	items := data.Items
	if items == nil {
		items = []map[string]interface{}{}
	}

	// recortar top
	if len(items) > top {
		items = items[:top]
	}

	// total = sum(item.total)
	total := 0
	for _, it := range items {
		if v, ok := it["total"]; ok {
			if n, ok := normalizeToInt(v); ok {
				total += n
			}
		}
	}

	// hoy: contar cuántas últimas visitas están en las últimas 24h (best-effort)
	hoy := 0
	now := time.Now()
	for _, it := range items {
		raw := normalizeToString(it["ultima_visita"])
		if raw == "" {
			continue
		}
		t := parseTimeValue(raw)
		if !t.IsZero() && now.Sub(t) <= 24*time.Hour {
			hoy++
		}
	}

	return map[string]interface{}{
		"total": total,
		"hoy":   hoy,
		"items": items,
	}, nil
}

type postulacionRecord struct {
	Id                int64           `json:"Id"`
	EstudianteId      int64           `json:"EstudianteId"`
	OfertaPasantiaId  json.RawMessage `json:"OfertaPasantiaId"`
	EstadoPostulacion string          `json:"EstadoPostulacion"`
	FechaPostulacion  string          `json:"FechaPostulacion"`
	EnlaceDocHv       string          `json:"EnlaceDocHv"`
	FechaEstado       time.Time       `json:"FechaEstado"`
}

// PerfilRecord represents a student profile stored in castor_crud.
type PerfilRecord struct {
	Id                   int
	TerceroId            int
	ProyectoCurricularId int
	Visible              bool
	Resumen              string
	Habilidades          string
	CvDocumentoRaw       json.RawMessage
	FechaCreacion        string
	FechaModificacion    string

	// Additional raw fields
	Extra map[string]interface{}
}

// PerfilVisita represents a visit to a student's profile.
type PerfilVisita struct {
	Id          int64
	PerfilId    int
	TutorId     int
	FechaVisita time.Time
}

func mapPerfilRecord(raw map[string]interface{}) PerfilRecord {
	rec := PerfilRecord{}
	if v, ok := normalizeToInt(raw["Id"]); ok {
		rec.Id = v
	}
	if v, ok := normalizeToInt(raw["TerceroId"]); ok {
		rec.TerceroId = v
	}
	if v, ok := normalizeToInt(raw["ProyectoCurricularId"]); ok {
		rec.ProyectoCurricularId = v
	}
	if v, ok := raw["Visible"].(bool); ok {
		rec.Visible = v
	}
	if v, ok := raw["Resumen"].(string); ok {
		rec.Resumen = v
	}
	if v, ok := raw["Habilidades"].(string); ok {
		rec.Habilidades = v
	}
	if rawCv, ok := raw["CvDocumentoId"]; ok {
		rec.CvDocumentoRaw, _ = json.Marshal(rawCv)
	}
	if v, ok := raw["FechaCreacion"].(string); ok {
		rec.FechaCreacion = v
	}
	if v, ok := raw["FechaModificacion"].(string); ok {
		rec.FechaModificacion = v
	}
	rec.Extra = raw
	return rec
}

func mapPerfilVisita(raw map[string]interface{}) PerfilVisita {
	visita := PerfilVisita{}

	if v, ok := normalizeToInt64(raw["Id"]); ok {
		visita.Id = v
	}

	// CRUD retorna PerfilId (no PerfilEstudianteId)
	if v, ok := normalizeToInt(raw["PerfilId"]); ok {
		visita.PerfilId = v
	} else if v, ok := normalizeToInt(raw["PerfilEstudianteId"]); ok { // fallback
		visita.PerfilId = v
	}

	if v, ok := normalizeToInt(raw["TutorId"]); ok {
		visita.TutorId = v
	}

	// CRUD retorna Fecha (no FechaVisita)
	if fecha, ok := raw["Fecha"].(string); ok {
		visita.FechaVisita = parseTimeValue(fecha)
	} else if fecha, ok := raw["FechaVisita"].(string); ok { // fallback
		visita.FechaVisita = parseTimeValue(fecha)
	}

	return visita
}

func normalizeInvitacionFilterKey(key string) string {
	trimmed := strings.TrimSpace(key)
	switch strings.ToLower(trimmed) {
	case "perfilid", "perfil_id", "perfilestudianteid":
		return "PerfilEstudianteId"
	case "estado":
		return "Estado"
	case "tutorid", "tutor_id":
		return "TutorId"
	default:
		return trimmed
	}
}

func normalizeToInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case json.Number:
		if parsed, err := strconv.ParseInt(v.String(), 10, 64); err == nil {
			return parsed, true
		}
	case string:
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			if parsed, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func normalizeToInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		if parsed, err := strconv.Atoi(v.String()); err == nil {
			return parsed, true
		}
	case string:
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			if parsed, err := strconv.Atoi(trimmed); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func parseTimeValue(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t
		}
	}
	return time.Time{}
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// normaliza cualquier valor a string (trim), útil para leer campos dinámicos del CRUD
func normalizeToString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		// representación compacta
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.Itoa(int(v))
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		// fallback: intenta serializar a JSON;
		if b, err := json.Marshal(v); err == nil {
			return strings.TrimSpace(string(b))
		}
		return fmt.Sprintf("%v", v)
	}
}
