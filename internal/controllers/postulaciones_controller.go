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

// PostulacionesController expone operaciones para gestionar postulaciones desde el tutor.
type PostulacionesController struct {
	rootcontrollers.BaseController
}

// GetByOferta lista las postulaciones de una oferta asociada al tutor.
// @Summary Listar postulaciones de la oferta
// @Description Ejemplo de respuesta: {"Success":true,"Status":200,"Message":"OK","Data":{"items":[{"id":101,"estado":"PPE_CTR","visto":true}],"total":1}}
// @Tags Postulaciones
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la oferta" Example(21)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *PostulacionesController) GetByOferta() {
	ofertaID, ok := c.parseOfertaID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	result, err := internalservices.ListarPostulaciones(c.Ctx.Request.Context(), tutorID, ofertaID)
	if err != nil {
		c.respondError(err, "error consultando postulaciones")
		return
	}

	resp := internalhelpers.Ok(result)
	c.writeJSON(resp.Status, resp)
}

// PostAccion ejecuta una acción sobre una postulación.
// @Summary Ejecutar acción sobre postulación
// @Description Acciones válidas: VISTO, DESCARTAR, PRESELECCIONAR, SELECCIONAR. Ejemplo de request: {"accion":"PRESELECCIONAR","comentario":"Avanza a entrevista"}
// @Tags Postulaciones
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la postulación" Example(101)
// @Param body body internaldto.PostulacionAccion true "Acción a ejecutar" Example({"accion":"PRESELECCIONAR","comentario":"Avanza a entrevista"})
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *PostulacionesController) PostAccion() {
	postulacionID, ok := c.parsePostulacionID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	var body internaldto.PostulacionAccion
	if err := c.ParseJSONBody(&body); err != nil {
		c.respondError(err, "cuerpo inválido")
		return
	}
	if strings.TrimSpace(body.Accion) == "" {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "accion requerida", nil), "accion requerida")
		return
	}

	result, err := internalservices.EjecutarAccionPostulacion(c.Ctx.Request.Context(), tutorID, postulacionID, body)
	if err != nil {
		c.respondError(err, "error ejecutando acción")
		return
	}

	resp := internalhelpers.Ok(result)
	resp.Message = "Acción registrada"
	c.writeJSON(resp.Status, resp)
}

// PutVisto marca una postulación como vista.
// @Summary Marcar postulación como vista
// @Description Marca la postulación como vista (PSRV_CTR) si está en PSPO_CTR.
// @Tags Postulaciones
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la postulación" Example(101)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 403 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *PostulacionesController) PutVisto() {
	postulacionID, ok := c.parsePostulacionID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	result, err := internalservices.MarcarPostulacionVista(c.Ctx.Request.Context(), tutorID, postulacionID)
	if err != nil {
		c.respondError(err, "error marcando postulación como vista")
		return
	}

	resp := internalhelpers.Ok(result)
	resp.Message = "Postulación marcada como vista"
	c.writeJSON(resp.Status, resp)
}

// PutAceptarSeleccion permite que el estudiante acepte la selección.
// @Summary Aceptar selección de postulación
// @Description Permite al estudiante aceptar la selección realizada por el tutor.
// @Tags Postulaciones
// @Accept json
// @Produce json
// @Param estudiante_id query int true "Id del estudiante" Example(4567)
// @Param id path int true "Id de la postulación" Example(101)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 403 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *PostulacionesController) PutAceptarSeleccion() {
	postulacionID, ok := c.parsePostulacionID()
	if !ok {
		return
	}
	estudianteID, ok := c.requireEstudiante()
	if !ok {
		return
	}

	if err := internalservices.AceptarSeleccion(c.Ctx.Request.Context(), estudianteID, postulacionID); err != nil {
		c.respondError(err, "no fue posible aceptar la selección")
		return
	}

	payload := map[string]interface{}{"postulacion_id": postulacionID}
	resp := internalhelpers.Ok(payload)
	resp.Message = "Selección aceptada"
	c.writeJSON(resp.Status, resp)
}

func (c *PostulacionesController) requireTutor() (int, bool) {
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

func (c *PostulacionesController) requireEstudiante() (int, bool) {
	raw := strings.TrimSpace(c.GetString("estudiante_id"))
	if raw == "" {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "estudiante_id requerido", nil), "estudiante_id requerido")
		return 0, false
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "estudiante_id inválido", err), "estudiante_id inválido")
		return 0, false
	}
	return id, true
}
func (c *PostulacionesController) parseOfertaID() (int, bool) {
	raw := strings.TrimSpace(c.Ctx.Input.Param(":id"))
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "id inválido", err), "id inválido")
		return 0, false
	}
	return val, true
}

func (c *PostulacionesController) parsePostulacionID() (int64, bool) {
	raw := strings.TrimSpace(c.Ctx.Input.Param(":id"))
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || val <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "id inválido", err), "id inválido")
		return 0, false
	}
	return val, true
}

func (c *PostulacionesController) respondError(err error, fallback string) {
	appErr := helpers.AsAppError(err, fallback)
	resp := internalhelpers.Fail(appErr.Status, appErr.Message)
	c.writeJSON(resp.Status, resp)
}

func (c *PostulacionesController) writeJSON(status int, payload internaldto.APIResponseDTO) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	_ = c.ServeJSON()
}
