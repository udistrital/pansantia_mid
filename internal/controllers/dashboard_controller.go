package controllers

import (
	"net/http"
	"strconv"
	"strings"

	rootcontrollers "github.com/udistrital/pasantia_mid/controllers"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	internalservices "github.com/udistrital/pasantia_mid/internal/services"
)

// DashboardController expone los dashboards por rol (estudiante / tutor).
type DashboardController struct{ rootcontrollers.BaseController }

// GET /v1/estudiantes/dashboard?estudiante_id=...
func (c *DashboardController) GetEstudiante() {
	estudianteID, ok := c.requireEstudiante()
	if !ok {
		return
	}
	data, err := internalservices.GetDashboardEstudiante(c.Ctx.Request.Context(), estudianteID)
	if err != nil {
		resp := internalhelpers.Fail(http.StatusInternalServerError, err.Error())
		c.writeJSON(resp.Status, resp)
		return
	}
	resp := internalhelpers.Ok(data)
	c.writeJSON(resp.Status, resp)
}

// GET /v1/tutores/dashboard?tutor_id=...
func (c *DashboardController) GetTutor() {
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}
	data, err := internalservices.GetDashboardTutor(c.Ctx.Request.Context(), tutorID)
	if err != nil {
		resp := internalhelpers.Fail(http.StatusInternalServerError, err.Error())
		c.writeJSON(resp.Status, resp)
		return
	}
	resp := internalhelpers.Ok(data)
	c.writeJSON(resp.Status, resp)
}

// --------------------- helpers locales ---------------------

func (c *DashboardController) requireEstudiante() (int, bool) {
	raw := strings.TrimSpace(c.GetString("estudiante_id")) // si ya usan JWT, aquí leer claim
	if raw == "" {
		resp := internalhelpers.Fail(http.StatusBadRequest, "estudiante_id requerido")
		c.writeJSON(resp.Status, resp)
		return 0, false
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		resp := internalhelpers.Fail(http.StatusBadRequest, "estudiante_id inválido")
		c.writeJSON(resp.Status, resp)
		return 0, false
	}
	return id, true
}

func (c *DashboardController) requireTutor() (int, bool) {
	raw := strings.TrimSpace(c.GetString("tutor_id")) // opcional: header X-Tutor-Id o JWT
	if raw == "" {
		resp := internalhelpers.Fail(http.StatusBadRequest, "tutor_id requerido")
		c.writeJSON(resp.Status, resp)
		return 0, false
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		resp := internalhelpers.Fail(http.StatusBadRequest, "tutor_id inválido")
		c.writeJSON(resp.Status, resp)
		return 0, false
	}
	return id, true
}

func (c *DashboardController) writeJSON(status int, payload interface{}) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	_ = c.ServeJSON()
}
