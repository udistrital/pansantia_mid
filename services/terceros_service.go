package services

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/models"
)

const (
	errMsgFindEmpresa          = "error consultando empresa en terceros"
	errMsgCreateEmpresa        = "error creando empresa en terceros"
	errMsgCreateTutor          = "error creando tutor externo en terceros"
	errMsgCreateDatoIdent      = "error registrando datos de identificación en terceros"
	errMsgCreateVinculacion    = "error creando vinculación en terceros"
	tercerosHTTPContentTypeKey = "Content-Type"
	tercerosHTTPContentTypeVal = "application/json"
)

// FindEmpresaByNIT consulta el CRUD de terceros y retorna el Id del tercero encontrado.
func FindEmpresaByNIT(nit string) (terceroId int, found bool, err error) {
	defer wrapTercerosError(&err, errMsgFindEmpresa)

	nitNorm := nitNormalize(nit)
	if nitNorm == "" {
		return 0, false, nil
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "datos_identificacion")
	params := url.Values{}
	params.Set("query", fmt.Sprintf("Numero:%s,Activo:true", nitNorm))
	params.Set("limit", "1")
	urlWithQuery := endpoint + "?" + params.Encode()
	fmt.Println("URL DE CONSULTA A DATOS IDENTIFICACIÓN", urlWithQuery)
	var payload []struct {
		TerceroId struct {
			Id int `json:"Id"`
		} `json:"TerceroId"`
	}

	headers := AddOASAuth(nil)
	if err = helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &payload, cfg.RequestTimeout, false); err != nil {
		if helpers.IsHTTPError(err, http.StatusNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if len(payload) == 0 || payload[0].TerceroId.Id == 0 {
		return 0, false, nil
	}
	return payload[0].TerceroId.Id, true, nil
}

// FindTerceroIDByDocumento devuelve el Id del tercero asociado al número de documento proporcionado.
func FindTerceroIDByDocumento(numero string) (int, error) {
	documento := strings.TrimSpace(numero)
	fmt.Println("Dato que entra a función que consulta a TERCEROS!", documento)
	if documento == "" {
		return 0, helpers.NewAppError(http.StatusBadRequest, "numero_documento requerido", nil)
	}

	cfg := GetConfig()
	fmt.Println("cfg TERCEROS!", cfg)
	endpoint := BuildURL(cfg.TercerosBaseURL, "datos_identificacion")
	params := url.Values{}
	params.Set("query", fmt.Sprintf("Numero:%s,Activo:true", documento))
	params.Set("limit", "1")
	urlWithQuery := endpoint + "?" + params.Encode()
	fmt.Println("URL Consulta Terceros!", urlWithQuery)
	var payload []struct {
		TerceroId models.FlexInt `json:"TerceroId"`
	}

	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &payload, cfg.RequestTimeout, true); err != nil {
		if helpers.IsHTTPError(err, http.StatusNotFound) {
			return 0, nil
		}
		return 0, err
	}
	fmt.Println("Respuesta Tercero ID", payload)
	if len(payload) == 0 {
		return 0, nil
	}

	terceroID := payload[0].TerceroId.Int()
	fmt.Println("Respuesta Tercero ID", terceroID)
	if terceroID == 0 {
		return 0, nil
	}
	return terceroID, nil
}

// CreateEmpresa crea un tercero de tipo empresa y su dato de identificación NIT.
func CreateEmpresa(in models.EmpresaInDTO) (empresaId int, err error) {
	defer wrapTercerosError(&err, errMsgCreateEmpresa)

	nitNorm := nitNormalize(in.NITSinDV)
	if nitNorm == "" {
		return 0, helpers.NewAppError(http.StatusBadRequest, "NIT inválido", nil)
	}
	if strings.TrimSpace(in.RazonSocial) == "" {
		return 0, helpers.NewAppError(http.StatusBadRequest, "razón social requerida", nil)
	}

	tipoDocumentoID := in.TipoDocumentoId
	if tipoDocumentoID == 0 {
		id, e := getTipoDocumentoID("NIT")
		if e != nil {
			return 0, e
		}
		tipoDocumentoID = id
	}

	tipoContribID := in.TipoContribuyenteId
	if tipoContribID == 0 {
		if id, e := getTipoContribuyenteID("P_JURIDICA"); e == nil && id > 0 {
			tipoContribID = id
		} else if id2, e2 := getTipoContribuyenteID("JURIDICA"); e2 == nil && id2 > 0 {
			tipoContribID = id2
		}
	}

	now := nowISO()
	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "tercero")

	body := map[string]interface{}{
		"NombreCompleto":    in.RazonSocial,
		"Activo":            true,
		"FechaCreacion":     now,
		"FechaModificacion": now,
	}
	if tipoContribID > 0 {
		body["TipoContribuyenteId"] = map[string]int{"Id": tipoContribID}
	}

	var response struct {
		Id int `json:"Id"`
	}

	headers := AddOASAuth(map[string]string{tercerosHTTPContentTypeKey: tercerosHTTPContentTypeVal})
	if err = helpers.DoJSONWithHeaders("POST", endpoint, headers, body, &response, cfg.RequestTimeout, false); err != nil {
		return 0, err
	}

	if response.Id == 0 {
		return 0, helpers.NewAppError(http.StatusBadGateway, "respuesta inválida al crear empresa", nil)
	}

	if _, err = CreateDatosIdentificacion(tipoDocumentoID, response.Id, nitNorm, true); err != nil {
		return 0, err
	}

	return response.Id, nil
}

// CreateTutorExterno crea el tercero persona natural y su dato de identificación.
func CreateTutorExterno(in models.TutorExternoInDTO) (tutorId int, err error) {
	defer wrapTercerosError(&err, errMsgCreateTutor)

	now := nowISO()
	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "tercero")

	body := map[string]interface{}{
		"NombreCompleto":  nombreCompleto(in.PrimerNombre, in.SegundoNombre, in.PrimerApellido, in.SegundoApellido),
		"PrimerNombre":    strings.TrimSpace(in.PrimerNombre),
		"SegundoNombre":   strings.TrimSpace(in.SegundoNombre),
		"PrimerApellido":  strings.TrimSpace(in.PrimerApellido),
		"SegundoApellido": strings.TrimSpace(in.SegundoApellido),
		"FechaNacimiento": strings.TrimSpace(in.FechaNacimiento),
		"Activo":          in.Activo,
		"TipoContribuyenteId": map[string]int{
			"Id": in.TipoContribuyenteId,
		},
		"FechaCreacion":     now,
		"FechaModificacion": now,
		"UsuarioWSO2":       strings.TrimSpace(in.UsuarioWSO2),
	}

	var response struct {
		Id int `json:"Id"`
	}

	headers := AddOASAuth(map[string]string{tercerosHTTPContentTypeKey: tercerosHTTPContentTypeVal})
	if err = helpers.DoJSONWithHeaders("POST", endpoint, headers, body, &response, cfg.RequestTimeout, false); err != nil {
		return 0, err
	}

	if response.Id == 0 {
		return 0, helpers.NewAppError(http.StatusBadGateway, "respuesta inválida al crear tutor externo", nil)
	}

	if _, err = CreateDatosIdentificacion(in.TipoDocumentoId, response.Id, strings.TrimSpace(in.NumeroDocumento), true); err != nil {
		return 0, err
	}

	return response.Id, nil
}

// CreateDatosIdentificacion registra un dato de identificación asociado a un tercero.
func CreateDatosIdentificacion(tipoDocumentoId, terceroId int, numero string, activo bool) (id int, err error) {
	defer wrapTercerosError(&err, errMsgCreateDatoIdent)

	now := nowISO()
	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "datos_identificacion")

	body := map[string]interface{}{
		"TipoDocumentoId": map[string]int{
			"Id": tipoDocumentoId,
		},
		"TerceroId": map[string]int{
			"Id": terceroId,
		},
		"Numero":            strings.TrimSpace(numero),
		"Activo":            activo,
		"FechaCreacion":     now,
		"FechaModificacion": now,
	}

	var response struct {
		Id int `json:"Id"`
	}

	headers := AddOASAuth(map[string]string{tercerosHTTPContentTypeKey: tercerosHTTPContentTypeVal})
	if err = helpers.DoJSONWithHeaders("POST", endpoint, headers, body, &response, cfg.RequestTimeout, false); err != nil {
		return 0, err
	}

	if response.Id == 0 {
		return 0, helpers.NewAppError(http.StatusBadGateway, "respuesta inválida al crear dato de identificación", nil)
	}

	return response.Id, nil
}

// CrearVinculacion registra la relación empresa - tutor externo.
func CrearVinculacion(empresaId, tutorId int) (vinculacionId int, err error) {
	defer wrapTercerosError(&err, errMsgCreateVinculacion)

	now := nowISO()
	cfg := GetConfig()
	endpoint := BuildURL(cfg.TercerosBaseURL, "vinculacion")

	body := map[string]interface{}{
		"TerceroPrincipalId": map[string]int{
			"Id": empresaId,
		},
		"TerceroRelacionadoId": map[string]int{
			"Id": tutorId,
		},
		"Activo":            true,
		"FechaCreacion":     now,
		"FechaModificacion": now,
	}

	var response struct {
		Id int `json:"Id"`
	}

	headers := AddOASAuth(map[string]string{tercerosHTTPContentTypeKey: tercerosHTTPContentTypeVal})
	if err = helpers.DoJSONWithHeaders("POST", endpoint, headers, body, &response, cfg.RequestTimeout, false); err != nil {
		return 0, err
	}

	if response.Id == 0 {
		return 0, helpers.NewAppError(http.StatusBadGateway, "respuesta inválida al crear vinculación", nil)
	}

	return response.Id, nil
}

// nowISO entrega la marca de tiempo UTC en formato RFC3339.
func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// nitNormalize elimina caracteres no numéricos del NIT y normaliza el texto.
func nitNormalize(s string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(s))
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range trimmed {
		if unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// nombreCompleto concatena los segmentos del nombre evitando espacios duplicados.
func nombreCompleto(pn, sn, pa, sa string) string {
	parts := []string{
		strings.TrimSpace(pn),
		strings.TrimSpace(sn),
		strings.TrimSpace(pa),
		strings.TrimSpace(sa),
	}
	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, " ")
}

func wrapTercerosError(err *error, message string) {
	if err == nil {
		return
	}
	if r := recover(); r != nil {
		*err = helpers.NewAppError(http.StatusBadGateway, message, fmt.Errorf("panic: %v", r))
		return
	}
	if *err == nil {
		return
	}
	appErr := helpers.AsAppError(*err, message)
	if appErr.Status == http.StatusInternalServerError {
		appErr.Status = http.StatusBadGateway
	}
	*err = appErr
}
