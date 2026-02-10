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

// ExplorarController expone el catálogo y acciones sobre perfiles de estudiantes.
type ExplorarController struct {
	rootcontrollers.BaseController
}

// GetCatalogo devuelve el catálogo paginado de perfiles visibles.
// @Summary Catálogo de estudiantes
// @Description Retorna la página solicitada del catálogo. Ejemplo de respuesta: {"Success":true,"Status":200,"Message":"OK","Data":{"items":[{"perfil_id":12,"resumen":"Ingeniero de sistemas","guardado":false}],"page":1,"size":10,"total":48}}
// @Tags Explorar
// @Accept json
// @Produce json
// @Param tutor_id query int false "Id del tutor (para marcar guardados)" Example(7890)
// @Param pc_id query int false "Filtrar por proyecto curricular" Example(57)
// @Param skills query string false "Filtrar por habilidades (icontains)" Example("excel")
// @Param q query string false "Texto libre" Example("industrial")
// @Param page query int false "Página (>=1)" Example(1)
// @Param size query int false "Tamaño de página (<=100)" Example(10)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/explorar/estudiantes [get]
func (c *ExplorarController) GetCatalogo() {
	pageStr := c.GetString("page")
	sizeStr := c.GetString("size")
	page, size := internalhelpers.ParsePageSize(pageStr, sizeStr)

	var pcID *int
	if raw := strings.TrimSpace(c.GetString("pc_id")); raw != "" {
		val, err := strconv.Atoi(raw)
		if err != nil || val <= 0 {
			c.respondError(helpers.NewAppError(http.StatusBadRequest, "pc_id inválido", err), "pc_id inválido")
			return
		}
		pcID = &val
	}

	filters := internalservices.CatalogoFilters{
		ProyectoCurricularID: pcID,
		Skills:               strings.TrimSpace(c.GetString("skills")),
		Query:                strings.TrimSpace(c.GetString("q")),
		Page:                 page,
		Size:                 size,
	}

	tutorID := optionalTutorID(c)
	result, err := internalservices.Catalogo(c.Ctx, filters, tutorID)
	if err != nil {
		c.respondError(err, "error consultando catálogo")
		return
	}

	resp := internalhelpers.Ok(result)
	c.writeJSON(resp.Status, resp)
}

// GetPerfil devuelve el detalle de un perfil.
// @Summary Detalle de perfil de estudiante
// @Description Retorna información del perfil incluyendo si está guardado por el tutor.
// @Tags Explorar
// @Accept json
// @Produce json
// @Param tutor_id query int false "Id del tutor (para marcar guardado)" Example(7890)
// @Param perfil_id path int true "Id del perfil" Example(12)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/explorar/estudiantes/{perfil_id} [get]
func (c *ExplorarController) GetPerfil() {
	perfilID, ok := c.parsePerfilID()
	if !ok {
		return
	}

	tutorID := optionalTutorID(c)
	detalle, err := internalservices.DetallePerfil(c.Ctx, perfilID, tutorID)
	if err != nil {
		c.respondError(err, "error consultando perfil")
		return
	}

	resp := internalhelpers.Ok(detalle)
	c.writeJSON(resp.Status, resp)
}

// PostGuardar registra el bookmark de un tutor sobre un perfil.
// @Summary Guardar perfil para tutor
// @Description Marca el perfil como guardado. Ejemplo de respuesta: {"Success":true,"Status":200,"Message":"OK","Data":{"message":"Perfil guardado"}}
// @Tags Explorar
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param perfil_id path int true "Id del perfil" Example(12)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/explorar/estudiantes/{perfil_id}/guardar [post]
func (c *ExplorarController) PostGuardar() {
	perfilID, ok := c.parsePerfilID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	if err := internalservices.GuardarPerfil(c.Ctx, tutorID, perfilID); err != nil {
		c.respondError(err, "error guardando perfil")
		return
	}

	resp := internalhelpers.Ok(map[string]string{"message": "Perfil guardado"})
	c.writeJSON(resp.Status, resp)
}

// DeleteGuardar elimina el bookmark del tutor sobre un perfil.
// @Summary Eliminar bookmark de perfil
// @Description Remueve el guardado de un perfil asociado al tutor.
// @Tags Explorar
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param perfil_id path int true "Id del perfil" Example(12)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/explorar/estudiantes/{perfil_id}/guardar [delete]
func (c *ExplorarController) DeleteGuardar() {
	perfilID, ok := c.parsePerfilID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	if err := internalservices.EliminarBookmark(c.Ctx, tutorID, perfilID); err != nil {
		c.respondError(err, "error eliminando bookmark")
		return
	}

	resp := internalhelpers.Ok(map[string]string{"message": "Bookmark eliminado"})
	c.writeJSON(resp.Status, resp)
}

// PostVisita registra la visita de un tutor a un perfil.
// @Summary Registrar visita de perfil
// @Description Registra el evento de visita para métricas. Ejemplo de respuesta: {"Success":true,"Status":200,"Message":"OK","Data":{"message":"Visita registrada"}}
// @Tags Explorar
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param perfil_id path int true "Id del perfil" Example(12)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// @router /v1/explorar/estudiantes/{perfil_id}/visita [post]
func (c *ExplorarController) PostVisita() {
	perfilID, ok := c.parsePerfilID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	if err := internalservices.RegistrarVisita(c.Ctx, tutorID, perfilID); err != nil {
		c.respondError(err, "error registrando visita")
		return
	}

	resp := internalhelpers.Ok(map[string]string{"message": "Visita registrada"})
	c.writeJSON(resp.Status, resp)
}

func (c *ExplorarController) parsePerfilID() (int, bool) {
	raw := strings.TrimSpace(c.Ctx.Input.Param(":perfil_id"))
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "perfil_id inválido", err), "perfil_id inválido")
		return 0, false
	}
	return val, true
}

func (c *ExplorarController) requireTutor() (int, bool) {
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

func optionalTutorID(c *ExplorarController) int {
	raw := strings.TrimSpace(c.GetString("tutor_id"))
	if raw == "" {
		return 0
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func (c *ExplorarController) respondError(err error, fallback string) {
	appErr := helpers.AsAppError(err, fallback)
	resp := internalhelpers.Fail(appErr.Status, appErr.Message)
	c.writeJSON(resp.Status, resp)
}

func (c *ExplorarController) writeJSON(status int, payload internaldto.APIResponseDTO) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	_ = c.ServeJSON()
}
