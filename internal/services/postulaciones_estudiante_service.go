package services

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/internal/clients"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	rootservices "github.com/udistrital/pasantia_mid/services"

	"github.com/beego/beego/v2/server/web/context"
)

const postulacionResource = "postulacion"

// PostularOferta registra la postulación del estudiante a una oferta.
func PostularOferta(ctx *context.Context, estudianteID int, ofertaID int64) (map[string]interface{}, error) {
	if estudianteID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "estudiante_id invalido", nil)
	}
	if ofertaID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "oferta_id invalido", nil)
	}

	stdCtx := requestContext(ctx)
	crud := clients.CastorCRUD()
	perfil, err := crud.GetPerfilByTerceroID(stdCtx, estudianteID)
	if err != nil {
		return nil, err
	}
	if perfil == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "perfil no encontrado", nil)
	}

	exists, err := existsPostulacion(estudianteID, ofertaID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, helpers.NewAppError(http.StatusConflict, "ya existe postulacion", nil)
	}

	body := map[string]interface{}{
		"EstudianteId":      estudianteID,
		"OfertaPasantiaId":  map[string]interface{}{"Id": ofertaID},
		"EstadoPostulacion": "PSPO_CTR",
		"FechaPostulacion":  nowISO(),
		"FechaEstado":       nowISO(),
	}

	if cv := extractCvDocumentoID(perfil.CvDocumentoRaw); cv != "" {
		body["EnlaceDocHv"] = cv
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, postulacionResource)

	var created map[string]interface{}
	if err := helpers.DoJSON("POST", endpoint, body, &created, cfg.RequestTimeout); err != nil {
		return nil, helpers.AsAppError(err, "error creando postulacion")
	}

	id, _ := normalizeToInt64(created["Id"])

	return map[string]interface{}{
		"id":                id,
		"estudiante_id":     int64(estudianteID),
		"oferta_id":         ofertaID,
		"estado":            "PSPO_CTR",
		"fecha_postulacion": body["FechaPostulacion"],
	}, nil
}

// ListarMisPostulaciones lista las postulaciones del estudiante.
func ListarMisPostulaciones(ctx *context.Context, estudianteID int, estado string, pageStr string, sizeStr string) (map[string]interface{}, error) {
	if estudianteID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "estudiante_id invalido", nil)
	}

	page, size := internalhelpers.ParsePageSize(pageStr, sizeStr)

	filters := map[string]string{
		"estudiante_id": fmt.Sprint(estudianteID),
	}
	if strings.TrimSpace(estado) != "" {
		filters["estado_postulacion"] = strings.TrimSpace(estado)
	}

	stdCtx := requestContext(ctx)
	postulaciones, err := clients.CastorCRUD().ListPostulaciones(stdCtx, filters)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando postulaciones")
	}

	total := len(postulaciones)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}

	items := make([]map[string]interface{}, 0, end-start)
	for _, p := range postulaciones[start:end] {
		code := strings.ToUpper(strings.TrimSpace(p.EstadoPostulacion))
		estadoNombre := resolveEstadoNombreEst(code)

		items = append(items, map[string]interface{}{
			"id":                p.Id,
			"estudiante_id":     p.EstudianteId,
			"oferta_id":         p.OfertaId,
			"estado":            strings.TrimSpace(p.EstadoPostulacion),
			"fecha_postulacion": strings.TrimSpace(p.FechaPostulacion),
			"Estado": map[string]string{
				"code":   code,
				"nombre": estadoNombre,
			},
		})
	}

	return map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	}, nil
}

// GetMiPostulacionDetalle retorna el detalle de una postulación del estudiante.
func GetMiPostulacionDetalle(ctx stdctx.Context, estudianteID int, postulacionID int64) (map[string]interface{}, error) {
	if estudianteID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "estudiante_id invalido", nil)
	}
	if postulacionID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "postulacion_id invalido", nil)
	}

	post, err := clients.CastorCRUD().GetPostulacionByID(ctx, postulacionID)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando postulacion")
	}
	if post == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "postulacion no encontrada", nil)
	}
	if int(post.EstudianteId) != estudianteID {
		return nil, helpers.NewAppError(http.StatusForbidden, "no autorizado para consultar esta postulación", nil)
	}

	code := strings.ToUpper(strings.TrimSpace(post.EstadoPostulacion))
	estadoNombre := resolveEstadoNombreEst(code)

	out := map[string]interface{}{
		"id":                post.Id,
		"estudiante_id":     post.EstudianteId,
		"oferta_id":         post.OfertaId,
		"estado":            strings.TrimSpace(post.EstadoPostulacion),
		"fecha_postulacion": strings.TrimSpace(post.FechaPostulacion),
		"estado_det": map[string]string{
			"code":   code,
			"nombre": estadoNombre,
		},
	}

	if oferta, err := rootservices.GetOferta(post.OfertaId); err == nil && oferta != nil {
		out["oferta_resumen"] = map[string]interface{}{
			"id":     oferta.Id,
			"titulo": strings.TrimSpace(oferta.Titulo),
		}
	}

	return out, nil
}

func existsPostulacion(estudianteID int, ofertaID int64) (bool, error) {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, postulacionResource)
	values := url.Values{}
	values.Set("limit", "1")
	values.Set("query", fmt.Sprintf("EstudianteId:%d,OfertaPasantiaId.Id:%d", estudianteID, ofertaID))

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var records []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &records, cfg.RequestTimeout); err != nil {
		if helpers.IsHTTPError(err, http.StatusNotFound) {
			return false, nil
		}
		return false, helpers.AsAppError(err, "error consultando postulacion")
	}
	if len(records) == 0 {
		return false, nil
	}
	if id, ok := normalizeToInt64(records[0]["Id"]); ok && id > 0 {
		return true, nil
	}
	return true, nil
}

func extractCvDocumentoID(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var intVal int
	if err := json.Unmarshal(raw, &intVal); err == nil && intVal > 0 {
		return fmt.Sprint(intVal)
	}
	var strVal string
	if err := json.Unmarshal(raw, &strVal); err == nil {
		return strings.TrimSpace(strVal)
	}
	return ""
}

func resolveEstadoNombreEst(code string) string {
	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "PSRJ_CTR" {
		return "Descartada"
	}
	if c != "" {
		if par, err := internalhelpers.GetParametroByCodeNoCache(c); err == nil {
			if nombre := strings.TrimSpace(par.Nombre); nombre != "" {
				return nombre
			}
		}
	}
	return c
}
