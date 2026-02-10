package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/models"
)

// CreateOferta crea una oferta en el CRUD de Castor.
func CreateOferta(dto models.CreateOfertaDTO) (*models.Oferta, error) {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia")

	payload := map[string]interface{}{
		"Titulo":         dto.Titulo,
		"Descripcion":    dto.Descripcion,
		"Estado":         models.OfertaEstadoCreada,
		"EmpresaId":      dto.EmpresaId,
		"TutorExternoId": dto.TutorExternoId,
	}

	var created castorOferta
	if err := helpers.DoJSON("POST", endpoint, payload, &created, cfg.RequestTimeout); err != nil {
		return nil, err
	}
	oferta := mapCastorOferta(created)
	return &oferta, nil
}

// ListOfertas obtiene ofertas filtrando por query params directos.
func ListOfertas(filters map[string]string) ([]models.Oferta, error) {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia")

	values := buildOfertaQuery(filters)
	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var raw []castorOferta
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &raw, cfg.RequestTimeout); err != nil {
		return nil, err
	}

	ofertas := make([]models.Oferta, 0, len(raw))
	for _, item := range raw {
		ofertas = append(ofertas, mapCastorOferta(item))
	}
	return ofertas, nil
}

// GetOferta recupera el detalle de una oferta.
func GetOferta(id int64) (*models.Oferta, error) {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia", strconv.FormatInt(id, 10))

	var raw castorOferta
	if err := helpers.DoJSON("GET", endpoint, nil, &raw, cfg.RequestTimeout); err != nil {
		return nil, err
	}
	oferta := mapCastorOferta(raw)
	return &oferta, nil
}

// UpdateOferta actualiza una oferta en el CRUD preservando los campos existentes.
func UpdateOferta(id int64, dto models.UpdateOfertaDTO) (*models.Oferta, error) {
	if dto.Titulo == nil && dto.Descripcion == nil && dto.Estado == nil {
		return GetOferta(id)
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia", strconv.FormatInt(id, 10))

	var current ofertaCrudRecord
	if err := helpers.DoJSON("GET", endpoint, nil, &current, cfg.RequestTimeout); err != nil {
		fmt.Println("UpdateOferta GET error:", err)
		return nil, helpers.NewAppError(http.StatusInternalServerError, "error actualizando oferta", err)
	}
	if int64(current.Id) != id {
		fmt.Println("UpdateOferta: oferta no encontrada", "id", id, "current", current.Id)
		return nil, helpers.NewAppError(http.StatusNotFound, "oferta no encontrada", nil)
	}

	if dto.Titulo != nil {
		current.Titulo = *dto.Titulo
	}
	if dto.Descripcion != nil {
		current.Descripcion = *dto.Descripcion
	}
	if dto.Estado != nil {
		current.Estado = strings.ToUpper(strings.TrimSpace(*dto.Estado))
	}

	var updated ofertaCrudRecord
	if err := helpers.DoJSON("PUT", endpoint, current, &updated, cfg.RequestTimeout); err != nil {
		fmt.Println("UpdateOferta PUT error:", err)
		return nil, helpers.NewAppError(http.StatusInternalServerError, "error actualizando oferta", err)
	}

	return GetOferta(id)
}

// UpdateOfertaMerge actualiza una oferta preservando los campos existentes.
// Hace GET, aplica patch y luego PUT completo para evitar pérdida de datos.
func UpdateOfertaMerge(id int64, patch map[string]interface{}) (*models.Oferta, error) {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia", strconv.FormatInt(id, 10))

	var raw castorOferta
	if err := helpers.DoJSON("GET", endpoint, nil, &raw, cfg.RequestTimeout); err != nil {
		return nil, err
	}
	if raw.Id == 0 {
		return nil, helpers.NewAppError(http.StatusNotFound, "oferta no encontrada", nil)
	}

	titulo := strings.TrimSpace(raw.Titulo)
	if titulo == "" || raw.EmpresaId == 0 {
		fmt.Println("WARNING UpdateOfertaMerge: datos incompletos, se evita PUT", "id", id, "titulo", titulo, "empresa_id", raw.EmpresaId)
		return nil, helpers.NewAppError(http.StatusConflict, "oferta inválida para actualización", nil)
	}

	payload := map[string]interface{}{
		"Id":             raw.Id,
		"Titulo":         raw.Titulo,
		"Descripcion":    raw.Descripcion,
		"Estado":         strings.ToUpper(strings.TrimSpace(raw.Estado)),
		"EmpresaId":      raw.EmpresaId,
		"TutorExternoId": raw.TutorExternoId,
	}
	if strings.TrimSpace(raw.FechaPublicacion) != "" {
		payload["FechaPublicacion"] = strings.TrimSpace(raw.FechaPublicacion)
	}
	if len(raw.ProyectoCurricularIds) > 0 {
		payload["ProyectoCurricularIds"] = raw.ProyectoCurricularIds
	}

	applyPatch(payload, patch)

	var updated castorOferta
	if err := helpers.DoJSON("PUT", endpoint, payload, &updated, cfg.RequestTimeout); err != nil {
		return nil, err
	}
	result := mapCastorOferta(updated)
	return &result, nil
}

// ChangeOfertaEstado cambia el estado de la oferta.
func ChangeOfertaEstado(id int64, estado string) (*models.Oferta, error) {
	normalized := strings.ToUpper(strings.TrimSpace(estado))
	if normalized == "" {
		return nil, helpers.NewAppError(http.StatusBadRequest, "estado requerido", nil)
	}

	switch normalized {
	case models.OfertaEstadoCreada:
		// transiciones a estado base: sin efectos secundarios
	case models.OfertaEstadoCancelada:
		if err := cancelarOferta(id); err != nil {
			return nil, err
		}
	case models.OfertaEstadoEnCurso:
		if err := validarOfertaEnCurso(id); err != nil {
			return nil, err
		}
	case models.OfertaEstadoPausada:
		// transición a pausa sin efectos secundarios
	case "OPFIN_CTR":
		// transición a finalizada sin efectos secundarios
	default:
		return nil, helpers.NewAppError(http.StatusBadRequest, "estado de oferta no soportado", nil)
	}
	return UpdateOfertaMerge(id, map[string]interface{}{"Estado": normalized})
}

// ListOfertaCarreras trae las carreras asociadas a una oferta.
func ListOfertaCarreras(id int64) ([]map[string]interface{}, error) {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia", strconv.FormatInt(id, 10), "carreras")

	var response []map[string]interface{}
	if err := helpers.DoJSON("GET", endpoint, nil, &response, cfg.RequestTimeout); err != nil {
		return nil, err
	}
	return response, nil
}

// AddOfertaCarrera vincula una oferta con un proyecto curricular.
func AddOfertaCarrera(id int64, dto models.OfertaCarreraDTO) error {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia", strconv.FormatInt(id, 10), "carreras")

	body := map[string][]int64{
		"proyecto_curricular_ids": {dto.ProyectoCurricularId},
	}

	var response map[string]interface{}
	if err := helpers.DoJSON("POST", endpoint, body, &response, cfg.RequestTimeout); err != nil {
		return err
	}
	return nil
}

// RemoveOfertaCarrera elimina la relación oferta-proyecto curricular.
func RemoveOfertaCarrera(id int64, carreraId int64) error {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia", strconv.FormatInt(id, 10), "carreras", fmt.Sprintf("%d", carreraId))

	var response map[string]interface{}
	if err := helpers.DoJSON("DELETE", endpoint, nil, &response, cfg.RequestTimeout); err != nil {
		return err
	}
	return nil
}

type castorOferta struct {
	Id                    int64   `json:"Id"`
	Titulo                string  `json:"Titulo"`
	Descripcion           string  `json:"Descripcion"`
	Estado                string  `json:"Estado"`
	EmpresaId             int64   `json:"EmpresaId"`
	TutorExternoId        int64   `json:"TutorExternoId"`
	FechaPublicacion      string  `json:"FechaPublicacion"`
	ProyectoCurricularIds []int64 `json:"ProyectoCurricularIds"`
}

type ofertaCrudRecord struct {
	Id               int             `json:"Id"`
	Titulo           string          `json:"Titulo"`
	Descripcion      string          `json:"Descripcion"`
	Estado           string          `json:"Estado"`
	FechaPublicacion time.Time       `json:"FechaPublicacion"`
	EmpresaId        json.RawMessage `json:"EmpresaId"`
	TutorExternoId   json.RawMessage `json:"TutorExternoId"`
}

var ofertaFilterMap = map[string]string{
	"id":                "Id",
	"titulo":            "Titulo",
	"descripcion":       "Descripcion",
	"estado":            "Estado",
	"empresa_id":        "EmpresaId",
	"tutor_externo_id":  "TutorExternoId",
	"fecha_publicacion": "FechaPublicacion",
}

func buildOfertaQuery(filters map[string]string) url.Values {
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
		case "limit", "offset", "fields", "sortby", "order":
			values.Set(key, trimmed)
		case "query":
			// already handled
		default:
			if field, ok := ofertaFilterMap[strings.ToLower(key)]; ok {
				queryParts = append(queryParts, fmt.Sprintf("%s:%s", field, trimmed))
			}
		}
	}

	if len(queryParts) > 0 {
		values.Set("query", strings.Join(queryParts, ","))
	}
	return values
}

func mapCastorOferta(raw castorOferta) models.Oferta {
	return models.Oferta{
		Id:                    raw.Id,
		Titulo:                strings.TrimSpace(raw.Titulo),
		Descripcion:           strings.TrimSpace(raw.Descripcion),
		Estado:                strings.TrimSpace(raw.Estado),
		FechaPublicacion:      parseCastorDate(raw.FechaPublicacion),
		EmpresaId:             raw.EmpresaId,
		TutorExternoId:        raw.TutorExternoId,
		ProyectoCurricularIds: raw.ProyectoCurricularIds,
	}
}

func applyPatch(payload map[string]interface{}, patch map[string]interface{}) {
	if patch == nil || payload == nil {
		return
	}
	for key, value := range patch {
		if value == nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "id":
			payload["Id"] = value
		case "titulo":
			payload["Titulo"] = value
		case "descripcion":
			payload["Descripcion"] = value
		case "estado":
			if s, ok := value.(string); ok {
				trimmed := strings.TrimSpace(s)
				if trimmed != "" {
					payload["Estado"] = strings.ToUpper(trimmed)
				}
			} else {
				payload["Estado"] = value
			}
		case "empresaid", "empresa_id":
			payload["EmpresaId"] = value
		case "tutorexternoid", "tutor_externo_id":
			payload["TutorExternoId"] = value
		case "fechapublicacion", "fecha_publicacion":
			switch v := value.(type) {
			case time.Time:
				if !v.IsZero() {
					payload["FechaPublicacion"] = v.Format(time.RFC3339)
				}
			case string:
				trimmed := strings.TrimSpace(v)
				if trimmed != "" {
					payload["FechaPublicacion"] = trimmed
				}
			default:
				payload["FechaPublicacion"] = value
			}
		case "proyectocurricularids", "proyecto_curricular_ids":
			payload["ProyectoCurricularIds"] = value
		default:

		}
	}
}

func cancelarOferta(ofertaID int64) error {
	postulaciones, err := ListPostulacionesByOferta(ofertaID)
	if err != nil {
		return err
	}
	if len(postulaciones) == 0 {
		return nil
	}

	estadosActivos := map[string]struct{}{
		"PSPO_CTR": {},
		"PSRV_CTR": {},
		"PSPR_CTR": {},
		"PSAC_CTR": {},
	}
	estadoCierre := "PSCD_CTR"

	ids := make([]int64, 0, len(postulaciones))
	for _, p := range postulaciones {
		estado := strings.ToUpper(strings.TrimSpace(p.EstadoPostulacion))
		if _, ok := estadosActivos[estado]; !ok {
			continue
		}
		ids = append(ids, p.Id)
	}
	if len(ids) == 0 {
		return nil
	}
	return updatePostulacionEstadoBulk(ids, estadoCierre)
}

func validarOfertaEnCurso(ofertaID int64) error {
	postulaciones, err := ListPostulacionesByOferta(ofertaID)
	if err != nil {
		return err
	}

	var aceptada *models.Postulacion
	var descartables []int64

	for i := range postulaciones {
		p := postulaciones[i]
		switch strings.ToUpper(strings.TrimSpace(p.EstadoPostulacion)) {
		case models.PostEstadoAceptada:
			if aceptada != nil {
				return helpers.NewAppError(http.StatusConflict, "la oferta tiene más de una postulación aceptada", nil)
			}
			aceptada = &p
		case models.PostEstadoDescartada:
			continue
		default:
			descartables = append(descartables, p.Id)
		}
	}

	if aceptada == nil {
		return helpers.NewAppError(http.StatusConflict, "la oferta requiere una postulación aceptada para pasar a curso", nil)
	}

	if len(descartables) > 0 {
		if err := updatePostulacionEstadoBulk(descartables, models.PostEstadoDescartada); err != nil {
			return err
		}
	}

	return descartarPostulacionesDelEstudiante(*aceptada)
}

func descartarPostulacionesDelEstudiante(aceptada models.Postulacion) error {
	todas, err := ListPostulacionesByEstudiante(aceptada.EstudianteId)
	if err != nil {
		return err
	}

	var ids []int64
	for _, p := range todas {
		if p.Id == aceptada.Id {
			continue
		}
		if strings.EqualFold(p.EstadoPostulacion, models.PostEstadoDescartada) {
			continue
		}
		ids = append(ids, p.Id)
	}
	if len(ids) == 0 {
		return nil
	}
	return updatePostulacionEstadoBulk(ids, models.PostEstadoDescartada)
}

func parseCastorDate(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed
	}
	return time.Time{}
}
