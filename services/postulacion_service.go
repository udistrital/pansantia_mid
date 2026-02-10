package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/models"
)

// CreatePostulacion crea una postulación aplicando idempotencia por estudiante y oferta.
func CreatePostulacion(dto models.CreatePostulacionDTO) (*models.Postulacion, bool, error) {
	filters := map[string]string{
		"estudiante_id": strconv.FormatInt(dto.EstudianteId, 10),
		"oferta_id":     strconv.FormatInt(dto.OfertaId, 10),
		"limit":         "1",
	}

	existentes, err := ListPostulaciones(filters)
	if err != nil {
		return nil, false, err
	}
	for _, existente := range existentes {
		if !strings.EqualFold(existente.EstadoPostulacion, models.PostEstadoDescartada) {
			return &existente, false, nil
		}
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "postulacion")

	body := map[string]interface{}{
		"EstudianteId":      dto.EstudianteId,
		"OfertaPasantiaId":  map[string]int64{"Id": dto.OfertaId},
		"EstadoPostulacion": models.PostEstadoEnviada,
	}
	if enlace := strings.TrimSpace(dto.EnlaceDocHv); enlace != "" {
		body["EnlaceDocHv"] = enlace
	}

	var created castorPostulacion
	if err := helpers.DoJSON("POST", endpoint, body, &created, cfg.RequestTimeout); err != nil {
		return nil, false, err
	}

	postulacion := mapCastorPostulacion(created)
	return &postulacion, true, nil
}

// ListPostulaciones consulta el CRUD de postulaciones aplicando filtros directos.
func ListPostulaciones(filters map[string]string) ([]models.Postulacion, error) {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "postulacion")

	values := buildPostulacionQuery(filters)
	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var raw []castorPostulacion
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &raw, cfg.RequestTimeout); err != nil {
		return nil, err
	}

	result := make([]models.Postulacion, 0, len(raw))
	for _, item := range raw {
		result = append(result, mapCastorPostulacion(item))
	}
	return result, nil
}

// GetPostulacion retorna el detalle de una postulación por Id.
func GetPostulacion(id int64) (*models.Postulacion, error) {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "postulacion", strconv.FormatInt(id, 10))

	var raw castorPostulacion
	if err := helpers.DoJSON("GET", endpoint, nil, &raw, cfg.RequestTimeout); err != nil {
		return nil, err
	}

	postulacion := mapCastorPostulacion(raw)
	return &postulacion, nil
}

// ListPostulacionesByOferta devuelve postulaciones asociadas a una oferta.
func ListPostulacionesByOferta(ofertaID int64) ([]models.Postulacion, error) {
	filters := map[string]string{
		"oferta_id": strconv.FormatInt(ofertaID, 10),
		"limit":     "0",
	}
	return ListPostulaciones(filters)
}

// ListPostulacionesByEstudiante devuelve postulaciones asociadas a un estudiante.
func ListPostulacionesByEstudiante(estudianteID int64) ([]models.Postulacion, error) {
	filters := map[string]string{
		"estudiante_id": strconv.FormatInt(estudianteID, 10),
		"limit":         "0",
	}
	return ListPostulaciones(filters)
}

// AceptarPostulacion actualiza una postulación a aceptada y realiza las cascadas requeridas.
func AceptarPostulacion(id int64) (*models.Postulacion, error) {
	target, err := GetPostulacion(id)
	if err != nil {
		return nil, err
	}

	if err := updatePostulacionEstado(id, models.PostEstadoAceptada); err != nil {
		return nil, err
	}

	postulacionesOferta, err := ListPostulacionesByOferta(target.OfertaId)
	if err != nil {
		return nil, err
	}

	var aDescartar []int64
	for _, p := range postulacionesOferta {
		if p.Id == id {
			continue
		}
		if strings.EqualFold(p.EstadoPostulacion, models.PostEstadoDescartada) {
			continue
		}
		aDescartar = append(aDescartar, p.Id)
	}
	if err := updatePostulacionEstadoBulk(aDescartar, models.PostEstadoDescartada); err != nil {
		return nil, err
	}

	postulacionesEstudiante, err := ListPostulacionesByEstudiante(target.EstudianteId)
	if err != nil {
		return nil, err
	}

	aDescartar = aDescartar[:0]
	for _, p := range postulacionesEstudiante {
		if p.Id == id || p.OfertaId == target.OfertaId {
			continue
		}
		if strings.EqualFold(p.EstadoPostulacion, models.PostEstadoDescartada) {
			continue
		}
		aDescartar = append(aDescartar, p.Id)
	}
	if err := updatePostulacionEstadoBulk(aDescartar, models.PostEstadoDescartada); err != nil {
		return nil, err
	}

	return GetPostulacion(id)
}

// SeleccionarPostulacion: Tutor marca como SELECCIONADA (PSSE_CTR).
// No ejecuta cascadas (las cascadas van cuando el estudiante acepta).
func SeleccionarPostulacion(id int64) (*models.Postulacion, error) {
	if err := updatePostulacionEstado(id, models.PostEstadoSeleccionada); err != nil {
		return nil, err
	}
	return GetPostulacion(id)
}

// DescartarPostulacion marca una postulación como descartada.
func DescartarPostulacion(id int64) (*models.Postulacion, error) {
	if err := updatePostulacionEstado(id, models.PostEstadoDescartada); err != nil {
		return nil, err
	}
	return GetPostulacion(id)
}

type castorPostulacion struct {
	Id                int64           `json:"id"`
	EstudianteId      int64           `json:"estudiante_id"`
	OfertaPasantiaId  json.RawMessage `json:"oferta_pasantia_id"`
	EstadoPostulacion string          `json:"estado_postulacion"`
	FechaPostulacion  string          `json:"fecha_postulacion"`
	EnlaceDocHv       string          `json:"enlace_doc_hv"`
	FechaEstado       *string         `json:"fecha_estado,omitempty"`
}

func buildPostulacionQuery(filters map[string]string) url.Values {
	values := url.Values{}
	values.Set("limit", "0")

	var queryParts []string
	if raw := strings.TrimSpace(filters["query"]); raw != "" {
		queryParts = append(queryParts, raw)
	}

	for key, value := range filters {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		switch strings.ToLower(key) {
		case "estudiante_id":
			queryParts = append(queryParts, fmt.Sprintf("EstudianteId:%s", trimmed))
		case "oferta_id":
			queryParts = append(queryParts, fmt.Sprintf("OfertaPasantiaId.Id:%s", trimmed))
		case "estado_postulacion":
			queryParts = append(queryParts, fmt.Sprintf("EstadoPostulacion__iexact:%s", trimmed))
		case "id":
			queryParts = append(queryParts, fmt.Sprintf("Id:%s", trimmed))
		case "limit", "offset", "fields", "sortby", "order":
			values.Set(key, trimmed)
		case "query":
		}
	}

	if len(queryParts) > 0 {
		values.Set("query", strings.Join(queryParts, ","))
	}
	return values
}

func mapCastorPostulacion(raw castorPostulacion) models.Postulacion {
	return models.Postulacion{
		Id:                raw.Id,
		EstudianteId:      raw.EstudianteId,
		OfertaId:          extractOfertaId(raw.OfertaPasantiaId),
		EstadoPostulacion: strings.TrimSpace(raw.EstadoPostulacion),
		FechaPostulacion:  strings.TrimSpace(raw.FechaPostulacion),
		EnlaceDocHv:       strings.TrimSpace(raw.EnlaceDocHv),
	}
}

func extractOfertaId(raw json.RawMessage) int64 {
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
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
		if parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func updatePostulacionEstado(id int64, estado string) error {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "postulacion", strconv.FormatInt(id, 10))

	var raw castorPostulacion
	if err := helpers.DoJSON("GET", endpoint, nil, &raw, cfg.RequestTimeout); err != nil {
		return err
	}

	raw.EstadoPostulacion = strings.ToUpper(strings.TrimSpace(estado))
	if err := helpers.DoJSON("PUT", endpoint, raw, &raw, cfg.RequestTimeout); err != nil {
		return err
	}
	return nil
}

func updatePostulacionEstadoBulk(ids []int64, estado string) error {
	if len(ids) == 0 {
		return nil
	}
	unique := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		unique[id] = struct{}{}
	}
	for id := range unique {
		if err := updatePostulacionEstado(id, estado); err != nil {
			return err
		}
	}
	return nil
}
