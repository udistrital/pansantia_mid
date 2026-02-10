package controllers

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
	"unicode"

	midhelpers "github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/models"
	"github.com/udistrital/pasantia_mid/models/requestresponse"
	"github.com/udistrital/pasantia_mid/services"

	"github.com/udistrital/pasantia_mid/internal/dto"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	internalservices "github.com/udistrital/pasantia_mid/internal/services"

	"github.com/beego/beego/v2/server/web"
)

type TercerosController struct{ web.Controller }

// PostRegistrarTutorExterno registra un tutor externo con empresa asociada.
func (c *TercerosController) PostRegistrarTutorExterno() {
	var payload models.RegistrarTutorExternoInDTO
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil {
		resp := internalhelpers.Fail(http.StatusBadRequest, "JSON inválido")
		c.writeJSON(resp.Status, resp)
		return
	}

	nitNormalizado, err := validarEmpresa(&payload.Empresa)
	if err != nil {
		c.respondAppError(err, "validación de empresa")
		return
	}
	if err := validarTutor(payload.TutorExterno); err != nil {
		c.respondAppError(err, "validación de tutor")
		return
	}

	empresaID, found, err := services.FindEmpresaByNIT(nitNormalizado)
	if err != nil {
		c.respondAppError(err, "buscar empresa")
		return
	}

	payload.Empresa.NITSinDV = nitNormalizado
	payload.TutorExterno.NumeroDocumento = strings.TrimSpace(payload.TutorExterno.NumeroDocumento)
	payload.TutorExterno.UsuarioWSO2 = strings.TrimSpace(payload.TutorExterno.UsuarioWSO2)

	if !found {
		empresaID, err = services.CreateEmpresa(payload.Empresa)
		if err != nil {
			c.respondAppError(err, "crear empresa")
			return
		}
	}

	tutorID, err := services.CreateTutorExterno(payload.TutorExterno)
	if err != nil {
		c.respondAppError(err, "crear tutor externo")
		return
	}

	vinculacionID, err := services.CrearVinculacion(empresaID, tutorID)
	if err != nil {
		c.respondAppError(err, "crear vinculación")
		return
	}

	out := models.RegistrarTutorExternoOutDTO{
		EmpresaId:      empresaID,
		TutorExternoId: tutorID,
		VinculacionId:  vinculacionID,
	}
	resp := requestresponse.NewSuccess(http.StatusCreated, "Tutor externo registrado", out)
	c.writeJSON(resp.Status, resp)
}

// GetEmpresaByID consulta empresa por id.
func (c *TercerosController) GetEmpresaByID() {
	id, err := internalhelpers.ParamInt(c.Ctx, ":id")
	if err != nil || id <= 0 {
		resp := internalhelpers.Fail(http.StatusBadRequest, "id inválido")
		c.writeJSON(resp.Status, resp)
		return
	}

	empresa, err := internalservices.ObtenerEmpresaPorID(c.Ctx, id)
	if err != nil {
		resp := internalhelpers.Fail(http.StatusNotFound, err.Error())
		c.writeJSON(resp.Status, resp)
		return
	}

	resp := internalhelpers.Ok(empresa)
	c.writeJSON(resp.Status, resp)
}

// GetTutorByID consulta tutor externo por id.
func (c *TercerosController) GetTutorByID() {
	id, err := internalhelpers.ParamInt(c.Ctx, ":id")
	if err != nil || id <= 0 {
		resp := internalhelpers.Fail(http.StatusBadRequest, "id inválido")
		c.writeJSON(resp.Status, resp)
		return
	}

	tutor, err := internalservices.ObtenerTutorExternoPorID(c.Ctx, id)
	if err != nil {
		resp := internalhelpers.Fail(http.StatusNotFound, err.Error())
		c.writeJSON(resp.Status, resp)
		return
	}

	resp := internalhelpers.Ok(tutor)
	c.writeJSON(resp.Status, resp)
}

func (c *TercerosController) writeJSON(status int, payload dto.APIResponseDTO) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	_ = c.ServeJSON()
}

func (c *TercerosController) respondAppError(err error, defaultMsg string) {
	appErr := midhelpers.AsAppError(err, defaultMsg)
	resp := requestresponse.NewError(appErr.Status, appErr.Message, nil)
	c.writeJSON(appErr.Status, resp)
}

func validarEmpresa(in *models.EmpresaInDTO) (string, error) {
	if in == nil {
		return "", midhelpers.NewAppError(http.StatusBadRequest, "datos de empresa requeridos", nil)
	}
	in.RazonSocial = strings.TrimSpace(in.RazonSocial)
	if in.RazonSocial == "" {
		return "", midhelpers.NewAppError(http.StatusBadRequest, "razón social requerida", nil)
	}
	nitNorm := normalizarNIT(in.NITSinDV)
	if nitNorm == "" {
		return "", midhelpers.NewAppError(http.StatusConflict, "nit inválido", nil)
	}
	return nitNorm, nil
}

func validarTutor(in models.TutorExternoInDTO) error {
	if strings.TrimSpace(in.PrimerNombre) == "" || strings.TrimSpace(in.PrimerApellido) == "" {
		return midhelpers.NewAppError(http.StatusBadRequest, "primer nombre y primer apellido son requeridos", nil)
	}
	if strings.TrimSpace(in.NumeroDocumento) == "" {
		return midhelpers.NewAppError(http.StatusBadRequest, "numero_documento requerido", nil)
	}
	if trimmed := strings.TrimSpace(in.UsuarioWSO2); trimmed != "" {
		if _, err := mail.ParseAddress(trimmed); err != nil {
			return midhelpers.NewAppError(http.StatusBadRequest, "usuario_wso2 inválido", err)
		}
	}
	return nil
}

func normalizarNIT(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range input {
		if unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
