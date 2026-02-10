package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	rootcontrollers "github.com/udistrital/pasantia_mid/controllers"
	"github.com/udistrital/pasantia_mid/helpers"
	internaldto "github.com/udistrital/pasantia_mid/internal/dto"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	internalservices "github.com/udistrital/pasantia_mid/internal/services"
)

// EstudiantesController gestiona el perfil del estudiante .
type EstudiantesController struct {
	rootcontrollers.BaseController
}

// GetMiPerfil obtiene el perfil del estudiante .
// @Summary Gestión de perfil de Estudiante (sin elegibilidad)
// @Description Devuelve el perfil vigente del estudiante . Ejemplo de respuesta: {"Success":true,"Status":200,"Message":"OK","Data":{"perfil_id":12,"resumen":"Estudiante de ingeniería industrial","habilidades":["excel","sql"],"visible":true}}
// @Tags Estudiantes
// @Accept json
// @Produce json
// @Param tercero_id query int true "Id del tercero (estudiante)" Example(4567)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/estudiantes/perfil [get]
func (c *EstudiantesController) GetMiPerfil() {
	terceroID, ok := c.parseTerceroID()
	if !ok {
		return
	}

	perfil, err := internalservices.ObtenerPerfil(c.Ctx, terceroID)
	if err != nil {
		c.respondError(err, "error consultando perfil")
		return
	}

	resp := internalhelpers.Ok(perfil)
	c.writeJSON(resp.Status, resp)
}

// PostUpsertPerfil crea o actualiza el perfil del estudiante .
// @Summary Gestión de perfil de Estudiante (sin elegibilidad)
// @Description Crea o actualiza el perfil del estudiante. Ejemplo de request: {"proyecto_curricular_id":57,"resumen":"Busco práctica profesional","habilidades":"{\"stack\":[\"Go\",\"React\"]}","cv_documento_id":"faf26e1f-f50c-4f0b-943b-98c0b1cd87e7","visible":true}
// @Tags Estudiantes
// @Accept json
// @Produce json
// @Param body body internaldto.EstudiantePerfilUpsertReq true "Cuerpo del perfil" Example({"tercero_id":4567,"proyecto_curricular_id":57,"resumen":"Busco práctica profesional","habilidades":"{\"stack\":[\"Go\",\"React\"]}","cv_documento_id":"faf26e1f-f50c-4f0b-943b-98c0b1cd87e7","visible":true})
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/estudiantes/perfil [post]
func (c *EstudiantesController) PostUpsertPerfil() {
	var body internaldto.EstudiantePerfilUpsertReq
	if err := c.ParseJSONBody(&body); err != nil {
		c.respondError(err, "cuerpo inválido")
		return
	}

	terceroID, ok := c.parseBodyTerceroID(body.TerceroID)
	if !ok {
		return
	}

	perfil, err := internalservices.UpsertPerfil(c.Ctx, terceroID, body.EstudiantePerfilUpsert)
	if err != nil {
		c.respondError(err, "error guardando perfil")
		return
	}

	resp := internalhelpers.Ok(perfil)
	c.writeJSON(resp.Status, resp)
}

// PutActualizarPerfil actualiza parcialmente el perfil del estudiante .
// @Summary Gestión de perfil de Estudiante (sin elegibilidad)
// @Description Actualiza parcialmente el perfil. Ejemplo de request: {"resumen":"Disponible para etapa productiva","visible":false}
// @Tags Estudiantes
// @Accept json
// @Produce json
// @Param body body internaldto.EstudiantePerfilUpsertReq true "Cuerpo del perfil" Example({"tercero_id":4567,"resumen":"Disponible para etapa productiva","visible":false})
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/estudiantes/perfil [put]
func (c *EstudiantesController) PutActualizarPerfil() {
	var body internaldto.EstudiantePerfilUpsertReq
	if err := c.ParseJSONBody(&body); err != nil {
		c.respondError(err, "cuerpo inválido")
		return
	}

	terceroID, ok := c.parseBodyTerceroID(body.TerceroID)
	if !ok {
		return
	}

	perfil, err := internalservices.ActualizarPerfil(c.Ctx, terceroID, body.EstudiantePerfilUpsert)
	if err != nil {
		c.respondError(err, "error actualizando perfil")
		return
	}

	resp := internalhelpers.Ok(perfil)
	c.writeJSON(resp.Status, resp)
}

// PutVisibilidad actualiza el estado de visibilidad del perfil.
// @Summary Gestión de perfil de Estudiante (sin elegibilidad)
// @Description Actualiza únicamente la visibilidad. Ejemplo de request: {"visible":true}
// @Tags Estudiantes
// @Accept json
// @Produce json
// @Param body body internaldto.EstudiantePerfilVisibilidadReq true "Payload de visibilidad" Example({"tercero_id":4567,"visible":true})
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/estudiantes/perfil/visibilidad [put]
func (c *EstudiantesController) PutVisibilidad() {
	var body internaldto.EstudiantePerfilVisibilidadReq
	if err := c.ParseJSONBody(&body); err != nil {
		c.respondError(err, "cuerpo inválido")
		return
	}
	terceroID, ok := c.parseBodyTerceroID(body.TerceroID)
	if !ok {
		return
	}
	if body.Visible == nil {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "visible es requerido", nil), "visible es requerido")
		return
	}

	payload := internaldto.EstudiantePerfilUpsert{
		Visible: body.Visible,
	}
	perfil, err := internalservices.ActualizarPerfil(c.Ctx, terceroID, payload)
	if err != nil {
		c.respondError(err, "error actualizando visibilidad")
		return
	}

	resp := internalhelpers.Ok(perfil)
	c.writeJSON(resp.Status, resp)
}

// PutCV actualiza la referencia al documento de CV del perfil.
// @Summary Gestión de perfil de Estudiante (sin elegibilidad)
// @Description Actualiza el CV del perfil. Ejemplo de request: {"cv_documento_id":"faf26e1f-f50c-4f0b-943b-98c0b1cd87e7"}
// @Tags Estudiantes
// @Accept json
// @Produce json
// @Param body body internaldto.EstudiantePerfilCVReq true "Payload de CV" Example({"tercero_id":4567,"cv_documento_id":"faf26e1f-f50c-4f0b-943b-98c0b1cd87e7"})
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/estudiantes/perfil/cv [put]
func (c *EstudiantesController) PutCV() {
	var body internaldto.EstudiantePerfilCVReq
	if err := c.ParseJSONBody(&body); err != nil {
		c.respondError(err, "cuerpo inválido")
		return
	}
	terceroID, ok := c.parseBodyTerceroID(body.TerceroID)
	if !ok {
		return
	}
	if body.CVDocumentoID == nil {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "cv_documento_id es requerido", nil), "cv_documento_id es requerido")
		return
	}

	payload := internaldto.EstudiantePerfilUpsert{
		CVDocumentoID: body.CVDocumentoID,
	}
	perfil, err := internalservices.ActualizarPerfil(c.Ctx, terceroID, payload)
	if err != nil {
		c.respondError(err, "error actualizando cv")
		return
	}

	resp := internalhelpers.Ok(perfil)
	c.writeJSON(resp.Status, resp)
}

// PostPerfilPorDocumento identifica si existe un perfil en Castor a partir del documento.
// @Summary Consulta perfil de estudiante por documento
// @Description Recibe el número de cédula y retorna si el estudiante ya está registrado en Castor.
// @Tags Estudiantes
// @Accept json
// @Produce json
// @Param body body internaldto.EstudiantePerfilDocumentoReq true "Payload con número de documento"
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/estudiantes/perfil/consulta_documento [post]
func (c *EstudiantesController) PostPerfilPorDocumento() {
	var body internaldto.EstudiantePerfilDocumentoReq
	if err := c.ParseJSONBody(&body); err != nil {
		c.respondError(err, "cuerpo inválido")
		return
	}

	numero := strings.TrimSpace(body.NumeroDocumento)
	fmt.Println("Documento Estudiante", numero)
	if numero == "" {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "numero_documento requerido", nil), "numero_documento requerido")
		return
	}

	c.consultarPerfilPorDocumento(numero)
}

func (c *EstudiantesController) consultarPerfilPorDocumento(numero string) {
	result, err := internalservices.ConsultarPerfilPorDocumento(c.Ctx, numero)
	if err != nil {
		c.respondError(err, "error consultando perfil por documento")
		return
	}

	resp := internalhelpers.Ok(result)
	c.writeJSON(resp.Status, resp)
}

func (c *EstudiantesController) parseTerceroID() (int, bool) {
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

func (c *EstudiantesController) parseBodyTerceroID(raw *int) (int, bool) {
	if raw == nil {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "tercero_id requerido", nil), "tercero_id requerido")
		return 0, false
	}
	id := *raw
	if id <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "tercero_id inválido", nil), "tercero_id inválido")
		return 0, false
	}
	return id, true
}

func (c *EstudiantesController) respondError(err error, fallback string) {
	appErr := helpers.AsAppError(err, fallback)
	resp := internalhelpers.Fail(appErr.Status, appErr.Message)
	c.writeJSON(resp.Status, resp)
}

func (c *EstudiantesController) writeJSON(status int, payload internaldto.APIResponseDTO) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	_ = c.ServeJSON()
}

// --------------------------
// GET /v1/estudiantes/perfil/visitas
// Devuelve quién vio mi perfil (agregado por tutor)
// --------------------------
func (c *EstudiantesController) GetVisitas() {
	estudianteID, ok := c.requireEstudiante()
	if !ok {
		return
	}

	result, err := internalservices.ListarVisitasPorEstudiante(c.Ctx, estudianteID)
	if err != nil {
		resp := internalhelpers.Fail(http.StatusInternalServerError, err.Error())
		c.writeJSON(resp.Status, resp)
		return
	}

	resp := internalhelpers.Ok(result)
	c.writeJSON(resp.Status, resp)
}

// --------------------------
// Helpers locales
// --------------------------
func (c *EstudiantesController) requireEstudiante() (int, bool) {
	raw := strings.TrimSpace(c.GetString("estudiante_id")) // si ya tienes JWT, reemplaza por lectura de claim
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

// func (c *EstudiantesController) writeJSON(status int, payload interface{}) {
// 	c.Ctx.Output.SetStatus(status)
// 	c.Data["json"] = payload
// 	_ = c.ServeJSON()
// }
