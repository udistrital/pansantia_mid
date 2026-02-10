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

// OfertaPCController gestiona la relación N:M entre oferta y proyectos curriculares.
type OfertaPCController struct {
	rootcontrollers.BaseController
}

// GetList obtiene los proyectos curriculares asociados a una oferta.
// @Summary Listar proyectos curriculares de la oferta
// @Description Devuelve los proyectos vinculados a la oferta.
// @Tags Oferta-PC
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la oferta" Example(21)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *OfertaPCController) GetList() {
	ofertaID, ok := c.parseOfertaID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	result, err := internalservices.ListarOfertaProyectos(c.Ctx, tutorID, ofertaID)
	if err != nil {
		c.respondError(err, "error consultando proyectos curriculares")
		return
	}

	resp := internalhelpers.Ok(result)
	c.writeJSON(resp.Status, resp)
}

// PostBulk enlaza proyectos curriculares a una oferta.
// @Summary Asociar proyectos curriculares a la oferta
// @Description Ejemplo de request: {"proyectos_curriculares":[101,205]}
// @Tags Oferta-PC
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la oferta" Example(21)
// @Param body body map[string][]int true "Lista de proyectos curriculares" Example({"proyectos_curriculares":[101,205]})
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *OfertaPCController) PostBulk() {
	ofertaID, ok := c.parseOfertaID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	var body struct {
		Proyectos []int `json:"proyectos_curriculares"`
	}
	if err := c.ParseJSONBody(&body); err != nil {
		c.respondError(err, "cuerpo inválido")
		return
	}
	if len(body.Proyectos) == 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "proyectos_curriculares requerido", nil), "proyectos_curriculares requerido")
		return
	}

	result, err := internalservices.AgregarOfertaProyectos(c.Ctx, tutorID, ofertaID, body.Proyectos)
	if err != nil {
		c.respondError(err, "error asociando proyectos curriculares")
		return
	}

	resp := internalhelpers.Ok(result)
	resp.Message = "Proyectos curriculares asociados"
	c.writeJSON(resp.Status, resp)
}

// DeleteOne elimina la relación oferta-proyecto curricular.
// @Summary Eliminar proyecto curricular de la oferta
// @Description Remueve la relación N:M entre oferta y proyecto curricular.
// @Tags Oferta-PC
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la oferta" Example(21)
// @Param pcId path int true "Id del proyecto curricular" Example(101)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *OfertaPCController) DeleteOne() {
	ofertaID, ok := c.parseOfertaID()
	if !ok {
		return
	}
	pcID, ok := c.parsePCID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	if err := internalservices.EliminarOfertaProyecto(c.Ctx, tutorID, ofertaID, pcID); err != nil {
		c.respondError(err, "error eliminando proyecto curricular")
		return
	}

	resp := internalhelpers.Ok(map[string]string{"message": "Proyecto curricular eliminado"})
	c.writeJSON(resp.Status, resp)
}

func (c *OfertaPCController) requireTutor() (int, bool) {
	raw := strings.TrimSpace(c.GetString("tutor_id"))
	if raw == "" {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "tutor_id requerido", nil), "tutor_id requerido")
		return 0, false
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "tutor_id inválido", err), "tutor_id inválido")
		return 0, false
	}
	return id, true
}
func (c *OfertaPCController) parseOfertaID() (int, bool) {
	raw := strings.TrimSpace(c.Ctx.Input.Param(":id"))
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "id inválido", err), "id inválido")
		return 0, false
	}
	return val, true
}

func (c *OfertaPCController) parsePCID() (int, bool) {
	raw := strings.TrimSpace(c.Ctx.Input.Param(":pcId"))
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "pcId inválido", err), "pcId inválido")
		return 0, false
	}
	return val, true
}

func (c *OfertaPCController) respondError(err error, fallback string) {
	appErr := helpers.AsAppError(err, fallback)
	resp := internalhelpers.Fail(appErr.Status, appErr.Message)
	c.writeJSON(resp.Status, resp)
}

func (c *OfertaPCController) writeJSON(status int, payload internaldto.APIResponseDTO) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	_ = c.ServeJSON()
}
