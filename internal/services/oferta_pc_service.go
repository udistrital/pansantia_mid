package services

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/udistrital/pasantia_mid/helpers"
	rootservices "github.com/udistrital/pasantia_mid/services"

	"github.com/beego/beego/v2/server/web/context"
)

const (
	ofertaPCResource = "oferta_proyecto_curricular"
)

// ListarOfertaProyectos devuelve los proyectos curriculares asociados a la oferta.
func ListarOfertaProyectos(ctx *context.Context, tutorID, ofertaID int) (map[string]interface{}, error) {
	if err := validarTutoria(ofertaID, tutorID); err != nil {
		return nil, err
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, ofertaPCResource)
	values := url.Values{}
	values.Set("limit", "0")
	values.Set("query", fmt.Sprintf("OfertaPasantiaId.Id:%d", ofertaID))

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var records []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &records, cfg.RequestTimeout); err != nil {
		return nil, helpers.AsAppError(err, "error consultando proyectos curriculares de la oferta")
	}

	items := make([]map[string]interface{}, 0, len(records))
	for _, record := range records {
		if pcID, ok := normalizeToInt(record["ProyectoCurricularId"]); ok && pcID > 0 {
			items = append(items, map[string]interface{}{
				"proyecto_curricular_id": pcID,
			})
		}
	}

	return map[string]interface{}{
		"items": items,
		"total": len(items),
	}, nil
}

// AgregarOfertaProyectos crea relaciones oferta-proyecto evitando duplicados.
func AgregarOfertaProyectos(ctx *context.Context, tutorID, ofertaID int, proyectos []int) (map[string]interface{}, error) {
	if err := validarTutoria(ofertaID, tutorID); err != nil {
		return nil, err
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, ofertaPCResource)

	existing := existingProyectos(ofertaID)
	unique := make(map[int]struct{}, len(proyectos))
	var created []int

	for _, pc := range proyectos {
		if pc <= 0 {
			continue
		}
		if _, seen := unique[pc]; seen {
			continue
		}
		unique[pc] = struct{}{}
		if existing[pc] {
			continue
		}

		body := map[string]interface{}{
			"OfertaPasantiaId":     map[string]int{"Id": ofertaID},
			"ProyectoCurricularId": pc,
		}

		var resp map[string]interface{}
		if err := helpers.DoJSON("POST", endpoint, body, &resp, cfg.RequestTimeout); err != nil {
			if helpers.IsHTTPError(err, http.StatusConflict) {
				continue
			}
			return nil, helpers.AsAppError(err, "error asociando proyecto curricular")
		}
		created = append(created, pc)
	}

	return map[string]interface{}{
		"creados": created,
	}, nil
}

// EliminarOfertaProyecto borra la relación oferta-proyecto curricular.
func EliminarOfertaProyecto(ctx *context.Context, tutorID, ofertaID, pcID int) error {
	if err := validarTutoria(ofertaID, tutorID); err != nil {
		return err
	}

	recordID, err := obtenerRelacionID(ofertaID, pcID)
	if err != nil {
		return err
	}
	if recordID == 0 {
		return helpers.NewAppError(http.StatusNotFound, "relación oferta-proyecto no encontrada", nil)
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, ofertaPCResource, strconv.Itoa(recordID))
	return helpers.DoJSON("DELETE", endpoint, nil, nil, cfg.RequestTimeout)
}

func validarTutoria(ofertaID, tutorID int) error {
	oferta, err := rootservices.GetOferta(int64(ofertaID))
	if err != nil {
		return helpers.AsAppError(err, "error consultando oferta")
	}
	if oferta == nil {
		return helpers.NewAppError(http.StatusNotFound, "oferta no encontrada", nil)
	}
	if int(oferta.TutorExternoId) != tutorID {
		return helpers.NewAppError(http.StatusForbidden, "no autorizado para gestionar esta oferta", nil)
	}
	return nil
}

func existingProyectos(ofertaID int) map[int]bool {
	result := make(map[int]bool)
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, ofertaPCResource)
	values := url.Values{}
	values.Set("limit", "0")
	values.Set("query", fmt.Sprintf("OfertaPasantiaId.Id:%d", ofertaID))

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var records []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &records, cfg.RequestTimeout); err != nil {
		return result
	}
	for _, record := range records {
		if pcID, ok := normalizeToInt(record["ProyectoCurricularId"]); ok && pcID > 0 {
			result[pcID] = true
		}
	}
	return result
}

func obtenerRelacionID(ofertaID, pcID int) (int, error) {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, ofertaPCResource)

	values := url.Values{}
	values.Set("limit", "1")
	values.Set("query", fmt.Sprintf("OfertaPasantiaId.Id:%d,ProyectoCurricularId:%d", ofertaID, pcID))

	urlWithQuery := endpoint
	if encoded := values.Encode(); encoded != "" {
		urlWithQuery = endpoint + "?" + encoded
	}

	var records []map[string]interface{}
	if err := helpers.DoJSON("GET", urlWithQuery, nil, &records, cfg.RequestTimeout); err != nil {
		if helpers.IsHTTPError(err, http.StatusNotFound) {
			return 0, nil
		}
		return 0, helpers.AsAppError(err, "error consultando relación oferta-proyecto")
	}
	if len(records) == 0 {
		return 0, nil
	}
	if id, ok := normalizeToInt(records[0]["Id"]); ok {
		return id, nil
	}
	return 0, nil
}
