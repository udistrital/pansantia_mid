package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/models"
)

var (
	infoComplementariaCache sync.Map
	tipoContribuyenteCache  sync.Map
	tipoDocumentoCache      sync.Map
)

// RegisterTutorExterno orquesta el registro completo de empresa, tutor y vinculación.
func RegisterTutorExterno(payload models.RegistroTutorExternoDTO) (*models.RegistroExternoResponse, error) {
	if err := validateRegistroTutorPayload(payload); err != nil {
		return nil, err
	}

	empresa, err := findEmpresaByNITLegacy(payload.Empresa.NIT)
	if err != nil {
		return nil, err
	}
	if empresa == nil {
		empresa, err = createEmpresaLegacy(payload.Empresa.NIT, payload.Empresa.Nombre)
		if err != nil {
			return nil, err
		}
	}

	tutorPayload := models.CreateTutorExternoDTO{
		Nombres:        payload.Nombres,
		Apellidos:      payload.Apellidos,
		Identificacion: payload.Identificacion,
		EmpresaId:      empresa.Id,
		Correo:         payload.Correo,
		Telefono:       payload.Telefono,
	}
	tutor, err := CreateOrUpdateTutorExterno(tutorPayload)
	if err != nil {
		return nil, err
	}

	if payload.TerceroPersonaId != nil && *payload.TerceroPersonaId > 0 {
		if err := vincularExternoConEmpresaLegacy(*payload.TerceroPersonaId, empresa.Id); err != nil {
			return nil, err
		}
	}

	return &models.RegistroExternoResponse{
		Empresa: empresa,
		Tutor:   tutor,
	}, nil
}

// FindTutorExternoByIdentificacion recupera un tutor externo existente por su identificación.
func FindTutorExternoByIdentificacion(identificacion string) (*models.TutorExterno, error) {
	ident := strings.TrimSpace(identificacion)
	if ident == "" {
		return nil, nil
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "datos_identificacion")
	params := url.Values{}
	params.Set("query", fmt.Sprintf("Numero:%s,Activo:true", ident))
	params.Set("limit", "1")

	urlWithQuery := endpoint + "?" + params.Encode()
	var response []struct {
		Id        int `json:"Id"`
		TerceroId int `json:"TerceroId"`
	}
	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &response, cfg.RequestTimeout, true); err != nil {
		return nil, err
	}
	if len(response) == 0 {
		return nil, nil
	}

	tercero, err := getTerceroByID(response[0].TerceroId)
	if err != nil {
		return nil, err
	}

	correo, _ := getInfoComplementariaDato(tercero.Id, "CORREO")
	telefono, _ := getInfoComplementariaDato(tercero.Id, "TELEFONO")

	return buildTutorExterno(tercero, ident, correo, telefono), nil
}

// CreateOrUpdateTutorExterno asegura que exista un tutor externo asociado a la empresa.
func CreateOrUpdateTutorExterno(payload models.CreateTutorExternoDTO) (*models.TutorExterno, error) {
	existing, err := FindTutorExternoByIdentificacion(payload.Identificacion)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		existing.EmpresaId = payload.EmpresaId
		if strings.TrimSpace(payload.Correo) != "" {
			_ = ensureInfoComplementariaDato(int(existing.Id), "CORREO", payload.Correo)
			existing.Correo = strings.TrimSpace(payload.Correo)
		}
		if strings.TrimSpace(payload.Telefono) != "" {
			_ = ensureInfoComplementariaDato(int(existing.Id), "TELEFONO", payload.Telefono)
			existing.Telefono = strings.TrimSpace(payload.Telefono)
		}
		return existing, nil
	}

	tercero, err := createTutorTercero(payload)
	if err != nil {
		return nil, err
	}

	if err := createTutorIdentificacion(tercero.Id, payload.Identificacion); err != nil {
		return nil, err
	}

	if strings.TrimSpace(payload.Correo) != "" {
		_ = ensureInfoComplementariaDato(tercero.Id, "CORREO", payload.Correo)
	}
	if strings.TrimSpace(payload.Telefono) != "" {
		_ = ensureInfoComplementariaDato(tercero.Id, "TELEFONO", payload.Telefono)
	}

	tutor := buildTutorExterno(tercero, payload.Identificacion, payload.Correo, payload.Telefono)
	tutor.EmpresaId = payload.EmpresaId
	return tutor, nil
}

func createTutorTercero(payload models.CreateTutorExternoDTO) (*models.Tercero, error) {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "tercero")

	primerNombre, segundoNombre := splitNombre(payload.Nombres)
	primerApellido, segundoApellido := splitNombre(payload.Apellidos)

	nombreCompleto := strings.TrimSpace(strings.Join(filterEmpty([]string{
		payload.Nombres,
		payload.Apellidos,
	}), " "))

	body := map[string]interface{}{
		"NombreCompleto": nombreCompleto,
		"Activo":         true,
	}
	if primerNombre != "" {
		body["PrimerNombre"] = primerNombre
	}
	if segundoNombre != "" {
		body["SegundoNombre"] = segundoNombre
	}
	if primerApellido != "" {
		body["PrimerApellido"] = primerApellido
	}
	if segundoApellido != "" {
		body["SegundoApellido"] = segundoApellido
	}

	if tipoContribuyenteID, err := getTipoContribuyenteID("P_NATURAL"); err == nil && tipoContribuyenteID > 0 {
		body["TipoContribuyenteId"] = tipoContribuyenteID
	} else if err != nil {
		return nil, err
	}

	headers := AddOASAuth(map[string]string{"Content-Type": "application/json"})
	var created models.Tercero
	if err := helpers.DoJSONWithHeaders("POST", endpoint, headers, body, &created, cfg.RequestTimeout, true); err != nil {
		return nil, err
	}
	return &created, nil
}

func createTutorIdentificacion(terceroID int, numero string) error {
	cfg := GetConfig()
	tipoDocumentoID, err := getTipoDocumentoID("CC")
	if err != nil {
		return err
	}

	endpoint := BuildURL(cfg.TercerosBaseURL, "datos_identificacion")
	body := map[string]interface{}{
		"Numero":          strings.TrimSpace(numero),
		"Activo":          true,
		"TerceroId":       terceroID,
		"TipoDocumentoId": tipoDocumentoID,
	}
	headers := AddOASAuth(map[string]string{"Content-Type": "application/json"})
	var created map[string]interface{}
	return helpers.DoJSONWithHeaders("POST", endpoint, headers, body, &created, cfg.RequestTimeout, true)
}

func ensureInfoComplementariaDato(terceroID int, codigo, valor string) error {
	val := strings.TrimSpace(valor)
	if val == "" {
		return nil
	}

	infoID, err := getInfoComplementariaID(codigo)
	if err != nil {
		return err
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "info_complementaria_tercero")
	params := url.Values{}
	params.Set("query", fmt.Sprintf("TerceroId__Id:%d,InfoComplementariaId__Id:%d,Activo:true", terceroID, infoID))
	params.Set("limit", "1")

	urlWithQuery := endpoint + "?" + params.Encode()
	var existentes []struct {
		Id   int    `json:"Id"`
		Dato string `json:"Dato"`
	}
	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &existentes, cfg.RequestTimeout, true); err != nil {
		return err
	}

	formatted := formatDatoValor(val)
	if len(existentes) == 0 {
		body := map[string]interface{}{
			"TerceroId":            terceroID,
			"InfoComplementariaId": infoID,
			"Dato":                 formatted,
			"Activo":               true,
		}
		postHeaders := AddOASAuth(map[string]string{"Content-Type": "application/json"})
		var created map[string]interface{}
		return helpers.DoJSONWithHeaders("POST", endpoint, postHeaders, body, &created, cfg.RequestTimeout, true)
	}

	if normalizarDato(existentes[0].Dato) == val {
		return nil
	}

	updateEndpoint := BuildURL(cfg.TercerosBaseURL, "info_complementaria_tercero", fmt.Sprintf("%d", existentes[0].Id))
	body := map[string]interface{}{
		"TerceroId":            terceroID,
		"InfoComplementariaId": infoID,
		"Dato":                 formatted,
		"Activo":               true,
	}
	putHeaders := AddOASAuth(map[string]string{"Content-Type": "application/json"})
	var updated map[string]interface{}
	return helpers.DoJSONWithHeaders("PUT", updateEndpoint, putHeaders, body, &updated, cfg.RequestTimeout, true)
}

func getInfoComplementariaDato(terceroID int, codigo string) (string, error) {
	infoID, err := getInfoComplementariaID(codigo)
	if err != nil {
		return "", err
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "info_complementaria_tercero")
	params := url.Values{}
	params.Set("query", fmt.Sprintf("TerceroId__Id:%d,InfoComplementariaId__Id:%d,Activo:true", terceroID, infoID))
	params.Set("limit", "1")

	urlWithQuery := endpoint + "?" + params.Encode()
	var existentes []struct {
		Dato string `json:"Dato"`
	}
	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &existentes, cfg.RequestTimeout, true); err != nil {
		return "", err
	}
	if len(existentes) == 0 {
		return "", nil
	}
	return normalizarDato(existentes[0].Dato), nil
}

func getInfoComplementariaID(codigo string) (int, error) {
	key := strings.ToUpper(strings.TrimSpace(codigo))
	if value, ok := infoComplementariaCache.Load(key); ok {
		if id, okCast := value.(int); okCast {
			return id, nil
		}
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "info_complementaria")
	params := url.Values{}
	params.Set("query", fmt.Sprintf("CodigoAbreviacion:%s,Activo:true", key))
	params.Set("limit", "1")

	urlWithQuery := endpoint + "?" + params.Encode()
	var response []struct {
		Id int `json:"Id"`
	}
	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &response, cfg.RequestTimeout, true); err != nil {
		return 0, err
	}
	if len(response) == 0 {
		return 0, helpers.NewAppError(404, fmt.Sprintf("info complementaria %s no encontrada", key), nil)
	}
	infoComplementariaCache.Store(key, response[0].Id)
	return response[0].Id, nil
}

func getTipoContribuyenteID(codigo string) (int, error) {
	key := strings.ToUpper(strings.TrimSpace(codigo))
	if value, ok := tipoContribuyenteCache.Load(key); ok {
		if id, okCast := value.(int); okCast {
			return id, nil
		}
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "tipo_contribuyente")
	params := url.Values{}
	params.Set("query", fmt.Sprintf("CodigoAbreviacion:%s,Activo:true", key))
	params.Set("limit", "1")

	urlWithQuery := endpoint + "?" + params.Encode()
	var response []struct {
		Id int `json:"Id"`
	}
	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &response, cfg.RequestTimeout, true); err != nil {
		return 0, err
	}
	if len(response) == 0 {
		return 0, helpers.NewAppError(404, fmt.Sprintf("tipo contribuyente %s no encontrado", key), nil)
	}
	tipoContribuyenteCache.Store(key, response[0].Id)
	return response[0].Id, nil
}

func buildTutorExterno(tercero *models.Tercero, identificacion, correo, telefono string) *models.TutorExterno {
	if tercero == nil {
		return nil
	}

	nombres := strings.TrimSpace(strings.Join(filterEmpty([]string{
		tercero.PrimerNombre,
		tercero.SegundoNombre,
	}), " "))
	apellidos := strings.TrimSpace(strings.Join(filterEmpty([]string{
		tercero.PrimerApellido,
		tercero.SegundoApellido,
	}), " "))

	if nombres == "" && tercero.NombreCompleto != "" {
		partes := strings.Fields(tercero.NombreCompleto)
		if len(partes) > 0 {
			nombres = partes[0]
		}
		if len(partes) > 1 {
			apellidos = strings.Join(partes[1:], " ")
		}
	}

	return &models.TutorExterno{
		Id:             int64(tercero.Id),
		Nombres:        nombres,
		Apellidos:      apellidos,
		Identificacion: strings.TrimSpace(identificacion),
		Correo:         strings.TrimSpace(correo),
		Telefono:       strings.TrimSpace(telefono),
	}
}

func splitNombre(valor string) (string, string) {
	partes := strings.Fields(strings.TrimSpace(valor))
	if len(partes) == 0 {
		return "", ""
	}
	if len(partes) == 1 {
		return partes[0], ""
	}
	return partes[0], strings.Join(partes[1:], " ")
}

func filterEmpty(values []string) []string {
	var filtered []string
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return filtered
}

func formatDatoValor(valor string) string {
	payload := map[string]string{"dato": strings.TrimSpace(valor)}
	data, err := json.Marshal(payload)
	if err != nil {
		return strings.TrimSpace(valor)
	}
	return string(data)
}

func normalizarDato(raw string) string {
	if trimmed := strings.TrimSpace(raw); trimmed == "" {
		return ""
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		if dato, ok := parsed["dato"]; ok {
			return strings.TrimSpace(fmt.Sprintf("%v", dato))
		}
		if len(parsed) == 1 {
			for _, v := range parsed {
				return strings.TrimSpace(fmt.Sprintf("%v", v))
			}
		}
	}
	return strings.TrimSpace(raw)
}

func findEmpresaByNITLegacy(nit string) (*models.Tercero, error) {
	normalized := nitNormalize(nit)
	if normalized == "" {
		return nil, helpers.NewAppError(http.StatusBadRequest, "nit vacío", nil)
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "datos_identificacion")
	params := url.Values{}
	params.Set("query", fmt.Sprintf("Numero:%s,Activo:true", normalized))
	params.Set("limit", "1")
	urlWithQuery := endpoint + "?" + params.Encode()

	var datos []models.DatosIdentificacion
	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &datos, cfg.RequestTimeout, true); err != nil {
		return nil, err
	}
	if len(datos) == 0 {
		return nil, nil
	}

	tercero, err := getTerceroByID(datos[0].TerceroId)
	if err != nil {
		return nil, err
	}
	return tercero, nil
}

func createEmpresaLegacy(nit, nombre string) (*models.Tercero, error) {
	normalized := nitNormalize(nit)
	if normalized == "" {
		return nil, helpers.NewAppError(http.StatusBadRequest, "NIT requerido", nil)
	}
	if strings.TrimSpace(nombre) == "" {
		return nil, helpers.NewAppError(http.StatusBadRequest, "Nombre de empresa requerido", nil)
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "tercero")

	body := map[string]interface{}{
		"NombreCompleto": nombre,
		"Activo":         true,
	}

	headers := AddOASAuth(map[string]string{"Content-Type": "application/json"})
	var created models.Tercero
	if err := helpers.DoJSONWithHeaders("POST", endpoint, headers, body, &created, cfg.RequestTimeout, true); err != nil {
		return nil, err
	}

	tipoDocumentoID, err := getTipoDocumentoID("NIT")
	if err != nil {
		return nil, err
	}

	datosEndpoint := BuildURL(cfg.TercerosBaseURL, "datos_identificacion")
	datosBody := map[string]interface{}{
		"Numero":          normalized,
		"Activo":          true,
		"TerceroId":       created.Id,
		"TipoDocumentoId": tipoDocumentoID,
	}
	if err := helpers.DoJSONWithHeaders("POST", datosEndpoint, headers, datosBody, &map[string]interface{}{}, cfg.RequestTimeout, true); err != nil {
		return nil, err
	}

	return &created, nil
}

func vincularExternoConEmpresaLegacy(terceroPersonaId, terceroEmpresaId int) error {
	if terceroPersonaId == 0 || terceroEmpresaId == 0 {
		return helpers.NewAppError(http.StatusBadRequest, "Ids de tercero inválidos", nil)
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "vinculacion")

	body := map[string]interface{}{
		"TerceroPrincipalId":   terceroEmpresaId,
		"TerceroRelacionadoId": terceroPersonaId,
		"Activo":               true,
	}

	headers := AddOASAuth(map[string]string{"Content-Type": "application/json"})
	return helpers.DoJSONWithHeaders("POST", endpoint, headers, body, &map[string]interface{}{}, cfg.RequestTimeout, true)
}

func getTerceroByID(id int) (*models.Tercero, error) {
	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "tercero", fmt.Sprintf("%d", id))

	var tercero models.Tercero
	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", endpoint, headers, nil, &tercero, cfg.RequestTimeout, true); err != nil {
		return nil, err
	}
	return &tercero, nil
}

func getTipoDocumentoID(codigo string) (int, error) {
	key := strings.ToUpper(strings.TrimSpace(codigo))
	if value, ok := tipoDocumentoCache.Load(key); ok {
		if id, okCast := value.(int); okCast {
			return id, nil
		}
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "tipo_documento")
	params := url.Values{}
	params.Set("query", fmt.Sprintf("CodigoAbreviacion:%s,Activo:true", key))
	params.Set("limit", "1")
	urlWithQuery := endpoint + "?" + params.Encode()

	var tipos []struct {
		Id int `json:"Id"`
	}
	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &tipos, cfg.RequestTimeout, true); err != nil {
		return 0, err
	}
	if len(tipos) == 0 {
		return 0, helpers.NewAppError(http.StatusNotFound, fmt.Sprintf("tipo documento %s no encontrado", key), nil)
	}

	tipoDocumentoCache.Store(key, tipos[0].Id)
	return tipos[0].Id, nil
}

func validateRegistroTutorPayload(payload models.RegistroTutorExternoDTO) error {
	if strings.TrimSpace(payload.Empresa.NIT) == "" || strings.TrimSpace(payload.Empresa.Nombre) == "" {
		return helpers.NewAppError(400, "Datos de empresa incompletos", nil)
	}
	if strings.TrimSpace(payload.Identificacion) == "" || strings.TrimSpace(payload.Nombres) == "" {
		return helpers.NewAppError(400, "Datos del tutor incompletos", nil)
	}
	return nil
}
