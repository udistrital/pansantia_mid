package services

import (
	"net/http"
	"strconv"

	"github.com/udistrital/pasantia_mid/helpers"
)

// GetTutorEmpresaActiva consulta la empresa activa asociada al tutor en Castor CRUD.
func GetTutorEmpresaActiva(tutorID int) (empresaID int, found bool, err error) {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "tutor_empresa", "activa", strconv.Itoa(tutorID))

	headers := AddOASAuth(nil)
	var response map[string]interface{}
	if err = helpers.DoJSONWithHeaders("GET", endpoint, headers, nil, &response, cfg.RequestTimeout, true); err != nil {
		if helpers.IsHTTPError(err, http.StatusNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}

	empresaID = extractEmpresaID(response)
	if empresaID == 0 {
		return 0, false, nil
	}
	return empresaID, true, nil
}

// UpsertTutorEmpresaActiva registra la empresa activa del tutor en Castor CRUD.
func UpsertTutorEmpresaActiva(tutorID, empresaID int) error {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.CastorCRUDBaseURL, "tutor_empresa", "activa")
	payload := map[string]interface{}{
		"tutor_id":   tutorID,
		"empresa_id": empresaID,
	}

	headers := AddOASAuth(nil)
	return helpers.DoJSONWithHeaders("POST", endpoint, headers, payload, &map[string]interface{}{}, cfg.RequestTimeout, true)
}

func extractEmpresaID(raw map[string]interface{}) int {
	if raw == nil {
		return 0
	}
	candidates := []string{"empresa_id", "EmpresaId", "empresaId"}
	for _, key := range candidates {
		if val, ok := raw[key]; ok {
			if parsed, ok := normalizeToInt(val); ok {
				return parsed
			}
		}
	}
	return 0
}
