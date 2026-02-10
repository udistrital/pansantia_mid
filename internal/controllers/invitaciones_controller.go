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

// InvitacionesController gestiona invitaciones entre tutores y estudiantes.
type InvitacionesController struct {
	rootcontrollers.BaseController
}

type terceroBody struct {
	TerceroID int `json:"tercero_id"`
}

// PostInvitar crea una invitación a partir de un tutor hacia un perfil.
func (c *InvitacionesController) PostInvitar() {
	perfilID, ok := c.parsePerfilID()
	if !ok {
		return
	}
	tutorID, ok := c.parseTutorID()
	if !ok {
		return
	}

	var body internaldto.InvitacionCreate
	if err := c.ParseJSONBody(&body); err != nil {
		c.respondError(err, "cuerpo inválido")
		return
	}
	if strings.TrimSpace(body.Mensaje) == "" {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "mensaje es requerido", nil), "mensaje es requerido")
		return
	}

	var ofertaID int64
	if body.OfertaPasantiaID != nil && *body.OfertaPasantiaID > 0 {
		ofertaID = *body.OfertaPasantiaID
	} else if body.OfertaID != nil && *body.OfertaID > 0 {
		ofertaID = *body.OfertaID
	}
	if ofertaID <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "oferta_pasantia_id es requerido", nil), "oferta_pasantia_id es requerido")
		return
	}
	body.OfertaID = &ofertaID
	body.OfertaPasantiaID = &ofertaID

	invitacion, err := internalservices.CrearInvitacion(c.Ctx.Request.Context(), tutorID, perfilID, body)
	if err != nil {
		c.respondError(err, "error creando invitación")
		return
	}

	resp := internalhelpers.Ok(invitacion)
	resp.Status = http.StatusCreated
	resp.Message = "Invitación enviada"
	c.writeJSON(resp.Status, resp)
}

// GetBandejaTutor lista las invitaciones asociadas a un tutor.
func (c *InvitacionesController) GetBandejaTutor() {
	tutorID, ok := c.parseTutorID()
	if !ok {
		return
	}

	page, size := internalhelpers.ParsePageSize(c.GetString("page"), c.GetString("size"))
	estado := strings.TrimSpace(c.GetString("estado"))

	result, err := internalservices.BandejaTutor(c.Ctx.Request.Context(), tutorID, estado, page, size)
	if err != nil {
		c.respondError(err, "error consultando invitaciones")
		return
	}

	resp := internalhelpers.Ok(result)
	c.writeJSON(resp.Status, resp)
}

// GetBandejaEstudiante lista las invitaciones del estudiante.
func (c *InvitacionesController) GetBandejaEstudiante() {
	estudianteID, ok := c.requireEstudiante()
	if !ok {
		return
	}

	estado := strings.TrimSpace(c.GetString("estado"))
	page, size := internalhelpers.ParsePageSize(c.GetString("page"), c.GetString("size"))

	out, err := internalservices.ListarInvitacionesDeEstudiante(c.Ctx.Request.Context(), estudianteID, estado, page, size)
	if err != nil {
		c.respondError(err, "error consultando invitaciones")
		return
	}
	resp := internalhelpers.Ok(out)
	c.writeJSON(resp.Status, resp)
}

// GetById obtiene el detalle de una invitación.
func (c *InvitacionesController) GetById() {
	invitacionID, ok := c.parseInvitacionID()
	if !ok {
		return
	}

	tutorRaw := strings.TrimSpace(c.GetString("tutor_id"))
	estudianteRaw := strings.TrimSpace(c.GetString("estudiante_id"))
	terceroRaw := strings.TrimSpace(c.GetString("tercero_id"))

	var tutorID, estudianteID, terceroID int
	var err error
	if tutorRaw != "" {
		if tutorID, err = strconv.Atoi(tutorRaw); err != nil || tutorID <= 0 {
			c.respondError(helpers.NewAppError(http.StatusBadRequest, "tutor_id inválido", err), "tutor_id inválido")
			return
		}
	}
	if estudianteRaw != "" {
		if estudianteID, err = strconv.Atoi(estudianteRaw); err != nil || estudianteID <= 0 {
			c.respondError(helpers.NewAppError(http.StatusBadRequest, "estudiante_id inválido", err), "estudiante_id inválido")
			return
		}
	}
	if terceroRaw != "" {
		if terceroID, err = strconv.Atoi(terceroRaw); err != nil || terceroID <= 0 {
			c.respondError(helpers.NewAppError(http.StatusBadRequest, "tercero_id inválido", err), "tercero_id inválido")
			return
		}
	}

	if tutorID <= 0 && estudianteID <= 0 && terceroID <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "tutor_id o estudiante_id o tercero_id requerido", nil), "tutor_id o estudiante_id o tercero_id requerido")
		return
	}

	data, err := internalservices.GetInvitacionDetalle(c.Ctx.Request.Context(), invitacionID, tutorID, estudianteID, terceroID)
	if err != nil {
		c.respondError(err, "error consultando invitación")
		return
	}

	resp := internalhelpers.Ok(data)
	c.writeJSON(resp.Status, resp)
}

// PutAceptar marca una invitación como aceptada.
func (c *InvitacionesController) PutAceptar() {
	invitacionID, ok := c.parseInvitacionID()
	if !ok {
		return
	}
	terceroID, ok := c.parseTerceroID()
	if !ok {
		return
	}

	invitacion, err := internalservices.AceptarInvitacion(c.Ctx.Request.Context(), invitacionID, terceroID)
	if err != nil {
		c.respondError(err, "error aceptando invitación")
		return
	}

	resp := internalhelpers.Ok(invitacion)
	resp.Message = "Invitación aceptada"
	c.writeJSON(resp.Status, resp)
}

// PutRechazar marca una invitación como rechazada.
func (c *InvitacionesController) PutRechazar() {
	invitacionID, ok := c.parseInvitacionID()
	if !ok {
		return
	}
	terceroID, ok := c.parseTerceroID()
	if !ok {
		return
	}

	invitacion, err := internalservices.RechazarInvitacion(c.Ctx.Request.Context(), invitacionID, terceroID)
	if err != nil {
		c.respondError(err, "error rechazando invitación")
		return
	}

	resp := internalhelpers.Ok(invitacion)
	resp.Message = "Invitación rechazada"
	c.writeJSON(resp.Status, resp)
}

func (c *InvitacionesController) parsePerfilID() (int, bool) {
	raw := strings.TrimSpace(c.Ctx.Input.Param(":perfil_id"))
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "perfil_id inválido", err), "perfil_id inválido")
		return 0, false
	}
	return val, true
}

func (c *InvitacionesController) parseInvitacionID() (int, bool) {
	raw := strings.TrimSpace(c.Ctx.Input.Param(":id"))
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "id inválido", err), "id inválido")
		return 0, false
	}
	return val, true
}

func (c *InvitacionesController) parseTutorID() (int, bool) {
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

func (c *InvitacionesController) parseTerceroID() (int, bool) {
	// 1) intentar body JSON
	if len(c.Ctx.Input.RequestBody) > 0 {
		var b terceroBody
		if err := c.ParseJSONBody(&b); err == nil {
			if b.TerceroID > 0 {
				return b.TerceroID, true
			}
		}
	}

	// 2) fallback query param
	raw := strings.TrimSpace(c.GetString("tercero_id"))
	if raw == "" {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "tercero_id requerido", nil), "tercero_id requerido")
		return 0, false
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "tercero_id inválido", err), "tercero_id inválido")
		return 0, false
	}
	return id, true
}

func (c *InvitacionesController) requireEstudiante() (int, bool) {
	raw := strings.TrimSpace(c.GetString("estudiante_id")) // si ya usan JWT, leer claim aquí
	id, err := strconv.Atoi(raw)
	if raw == "" || err != nil || id <= 0 {
		resp := internalhelpers.Fail(http.StatusBadRequest, "estudiante_id requerido/ inválido")
		c.writeJSON(resp.Status, resp)
		return 0, false
	}
	return id, true
}

func (c *InvitacionesController) respondError(err error, fallback string) {
	appErr := helpers.AsAppError(err, fallback)
	resp := internalhelpers.Fail(appErr.Status, appErr.Message)
	c.writeJSON(resp.Status, resp)
}

func (c *InvitacionesController) writeJSON(status int, payload internaldto.APIResponseDTO) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	_ = c.ServeJSON()
}
