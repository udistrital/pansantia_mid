package controllers

import (
	"net/http"
	"strconv"
	"strings"

	rootcontrollers "github.com/udistrital/pasantia_mid/controllers"
	"github.com/udistrital/pasantia_mid/helpers"
	internaldto "github.com/udistrital/pasantia_mid/internal/dto"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	internalservices "github.com/udistrital/pasantia_mid/internal/services"
)

// PostulacionesEstudianteController gestiona postulaciones desde la perspectiva del estudiante.
type PostulacionesEstudianteController struct {
	rootcontrollers.BaseController
}

// PostPostularOferta registra la postulación del estudiante a una oferta.
func (c *PostulacionesEstudianteController) PostPostularOferta() {
	ofertaID, ok := c.parseOfertaID()
	if !ok {
		return
	}
	estudianteID, ok := c.requireEstudiante()
	if !ok {
		return
	}

	result, err := internalservices.PostularOferta(c.Ctx, estudianteID, int64(ofertaID))
	if err != nil {
		c.respondError(err, "error postulando oferta")
		return
	}

	resp := internalhelpers.Ok(result)
	c.writeJSON(resp.Status, resp)
}

// GetMisPostulaciones lista las postulaciones del estudiante.
func (c *PostulacionesEstudianteController) GetMisPostulaciones() {
	estudianteID, ok := c.requireEstudiante()
	if !ok {
		return
	}

	estado := strings.TrimSpace(c.GetString("estado"))
	pageStr := c.GetString("page")
	sizeStr := c.GetString("size")
	out, err := internalservices.ListarMisPostulaciones(c.Ctx, estudianteID, estado, pageStr, sizeStr)
	if err != nil {
		c.respondError(err, "error consultando postulaciones")
		return
	}

	resp := internalhelpers.Ok(out)
	c.writeJSON(resp.Status, resp)
}

// GetById obtiene el detalle de una postulación del estudiante.
func (c *PostulacionesEstudianteController) GetById() {
	estudianteID, ok := c.requireEstudiante()
	if !ok {
		return
	}

	raw := strings.TrimSpace(c.Ctx.Input.Param(":id"))
	postulacionID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || postulacionID <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "id invalido", err), "id invalido")
		return
	}

	data, err := internalservices.GetMiPostulacionDetalle(c.Ctx.Request.Context(), estudianteID, postulacionID)
	if err != nil {
		c.respondError(err, "error consultando postulación")
		return
	}

	resp := internalhelpers.Ok(data)
	c.writeJSON(resp.Status, resp)
}

func (c *PostulacionesEstudianteController) requireEstudiante() (int, bool) {
	raw := strings.TrimSpace(c.GetString("estudiante_id"))
	if raw == "" {
		resp := internalhelpers.Fail(http.StatusBadRequest, "estudiante_id requerido")
		c.writeJSON(resp.Status, resp)
		return 0, false
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		resp := internalhelpers.Fail(http.StatusBadRequest, "estudiante_id invalido")
		c.writeJSON(resp.Status, resp)
		return 0, false
	}
	return id, true
}

func (c *PostulacionesEstudianteController) parseOfertaID() (int, bool) {
	raw := strings.TrimSpace(c.Ctx.Input.Param(":id"))
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "id invalido", err), "id invalido")
		return 0, false
	}
	return val, true
}

func (c *PostulacionesEstudianteController) respondError(err error, fallback string) {
	appErr := helpers.AsAppError(err, fallback)
	resp := internalhelpers.Fail(appErr.Status, appErr.Message)
	c.writeJSON(resp.Status, resp)
}

func (c *PostulacionesEstudianteController) writeJSON(status int, payload internaldto.APIResponseDTO) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	_ = c.ServeJSON()
}
