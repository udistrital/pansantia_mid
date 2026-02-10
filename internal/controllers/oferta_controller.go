package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	rootcontrollers "github.com/udistrital/pasantia_mid/controllers"
	"github.com/udistrital/pasantia_mid/helpers"
	internaldto "github.com/udistrital/pasantia_mid/internal/dto"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	internalservices "github.com/udistrital/pasantia_mid/internal/services"
	"github.com/udistrital/pasantia_mid/models"
)

// OfertaController expone operaciones de ofertas desde la perspectiva del tutor.
type OfertaController struct {
	rootcontrollers.BaseController
}

// CrearOfertaReq reexporta el payload esperado para crear ofertas con proyectos curriculares.
type CrearOfertaReq = internalservices.CrearOfertaReq

// @Summary Crear oferta con PCs asociados
// @Description Crea oferta en Castor_CRUD y asocia proyectos curriculares en lote.
// @Tags Ofertas
// @Accept json
// @Produce json
// @Param body body controllers.CrearOfertaReq true "Payload de creación"
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 409 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
// PostCrear crea una oferta y vincula proyectos curriculares en una sola solicitud.
func (c *OfertaController) PostCrear() {
	var req CrearOfertaReq
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		resp := internalhelpers.Fail(http.StatusBadRequest, "JSON inválido")
		c.writeJSON(resp.Status, resp)
		return
	}

	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	data, err := internalservices.CrearOfertaConPCs(c.Ctx.Request.Context(), tutorID, req)
	if err != nil {
		appErr := helpers.AsAppError(err, "error creando oferta")
		resp := internalhelpers.Fail(appErr.Status, appErr.Message)
		c.writeJSON(resp.Status, resp)
		return
	}

	resp := internalhelpers.Ok(data)
	c.writeJSON(resp.Status, resp)
}

// GetAbiertas lista las ofertas abiertas del tutor.
// @Summary Listar ofertas abiertas del tutor
// @Description Retorna las ofertas con estado abierto. Ejemplo de respuesta: {"Success":true,"Status":200,"Message":"OK","Data":{"items":[{"id":21,"titulo":"Pasantía QA"}],"total":1,"estado":"OPC_CTR"}}
// @Tags Ofertas
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *OfertaController) GetAbiertas() {
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	ofertas, err := internalservices.ListarOfertas(c.Ctx, tutorID, internalservices.OfertaEstadoAbierta)
	if err != nil {
		c.respondError(err, "error consultando ofertas abiertas")
		return
	}
	resp := internalhelpers.Ok(ofertas)
	c.writeJSON(resp.Status, resp)
}

// GetEnCurso lista las ofertas en curso del tutor.
// @Summary Listar ofertas en curso del tutor
// @Description Retorna ofertas con estado en curso.
// @Tags Ofertas
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *OfertaController) GetEnCurso() {
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	ofertas, err := internalservices.ListarOfertas(c.Ctx, tutorID, internalservices.OfertaEstadoEnCurso)
	if err != nil {
		c.respondError(err, "error consultando ofertas en curso")
		return
	}
	resp := internalhelpers.Ok(ofertas)
	c.writeJSON(resp.Status, resp)
}

// PutCancelar cancela una oferta.
// @Summary Cancelar oferta
// @Description Cambia la oferta a estado cancelado e impacta postulaciones relacionadas.
// @Tags Ofertas
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la oferta" Example(21)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *OfertaController) PutCancelar() {
	ofertaID, ok := c.parseOfertaID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	result, err := internalservices.CambiarEstadoOferta(c.Ctx, tutorID, ofertaID, internalservices.OfertaEstadoCancelada)
	if err != nil {
		c.respondError(err, "error cancelando oferta")
		return
	}
	resp := internalhelpers.Ok(result)
	resp.Message = "Oferta cancelada"
	c.writeJSON(resp.Status, resp)
}

// PutFinalizar marca una oferta como finalizada.
// @Summary Finalizar oferta
// @Description Establece el estado de la oferta como finalizada. Ejemplo de respuesta: {"Success":true,"Status":200,"Message":"Oferta finalizada","Data":{"estado":"OPFIN_CTR"}}
// @Tags Ofertas
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la oferta" Example(21)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *OfertaController) PutFinalizar() {
	resp := internalhelpers.Fail(http.StatusNotImplemented, "Acción no disponible por el momento")
	c.writeJSON(resp.Status, resp)
	return
}

// PutPausar pausa una oferta.
// @Summary Pausar oferta
// @Description Cambia la oferta a estado pausado. Ejemplo de respuesta: {"Success":true,"Status":200,"Message":"Oferta pausada","Data":{"estado":"OPPAU_CTR"}}
// @Tags Ofertas
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la oferta" Example(21)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *OfertaController) PutPausar() {
	c.Pausar()
}

// Pausar pausa una oferta.
// @Summary Pausar oferta
// @Description Cambia la oferta a estado pausado. Ejemplo de respuesta: {"Success":true,"Status":200,"Message":"Oferta pausada","Data":{"estado":"OPPAU_CTR"}}
// @Tags Ofertas
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la oferta" Example(21)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *OfertaController) Pausar() {
	ofertaID, ok := c.parseOfertaID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	result, err := internalservices.CambiarEstadoOferta(c.Ctx, tutorID, ofertaID, "pausar")
	if err != nil {
		c.respondError(err, "error pausando oferta")
		return
	}
	resp := internalhelpers.Ok(result)
	resp.Message = "Oferta pausada"
	c.writeJSON(resp.Status, resp)
}

// PutReactivar reactiva una oferta pausada.
// @Summary Reactivar oferta
// @Description Cambia la oferta a estado creada/abierta. Ejemplo de respuesta: {"Success":true,"Status":200,"Message":"Oferta reactivada","Data":{"estado":"OPC_CTR"}}
// @Tags Ofertas
// @Accept json
// @Produce json
// @Param tutor_id query int true "Id del tutor" Example(7890)
// @Param id path int true "Id de la oferta" Example(21)
// @Success 200 {object} internaldto.APIResponseDTO
// @Failure 400 {object} internaldto.APIResponseDTO
// @Failure 404 {object} internaldto.APIResponseDTO
// @Failure 500 {object} internaldto.APIResponseDTO
func (c *OfertaController) PutReactivar() {
	ofertaID, ok := c.parseOfertaID()
	if !ok {
		return
	}
	tutorID, ok := c.requireTutor()
	if !ok {
		return
	}

	result, err := internalservices.CambiarEstadoOferta(c.Ctx, tutorID, ofertaID, models.OfertaEstadoCreada)
	if err != nil {
		c.respondError(err, "error reactivando oferta")
		return
	}
	resp := internalhelpers.Ok(result)
	resp.Message = "Oferta reactivada"
	c.writeJSON(resp.Status, resp)
}

// GetListado lista ofertas aplicando filtros opcionales.
func (c *OfertaController) GetListado() {
	estados := strings.TrimSpace(c.GetString("estado"))
	tutorIDStr := strings.TrimSpace(c.GetString("tutor_id"))
	pcIDStr := strings.TrimSpace(c.GetString("pc_id"))
	estudianteIDStr := strings.TrimSpace(c.GetString("estudiante_id"))
	q := strings.TrimSpace(c.GetString("q"))
	page, _ := strconv.Atoi(c.GetString("page"))
	size, _ := strconv.Atoi(c.GetString("size"))
	sort := strings.TrimSpace(c.GetString("sort"))
	order := strings.TrimSpace(c.GetString("order"))
	excludePostuladas := strings.TrimSpace(c.GetString("exclude_postuladas"))
	exclude := excludePostuladas == "true" || excludePostuladas == "1" || strings.EqualFold(excludePostuladas, "yes")

	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	var tutorID int
	if tutorIDStr != "" {
		tutorID, _ = strconv.Atoi(tutorIDStr)
	}
	var pcID int
	if pcIDStr != "" {
		pcID, _ = strconv.Atoi(pcIDStr)
	}

	var estudianteID int
	if estudianteIDStr != "" {
		estudianteID, _ = strconv.Atoi(estudianteIDStr)
	}

	data, err := internalservices.ListarOfertasCatalogo(
		c.Ctx,
		estados,
		tutorID,
		pcID,
		estudianteID,
		q,
		page,
		size,
		sort,
		order,
		exclude,
	)
	if err != nil {
		c.respondError(err, "error listando ofertas")
		return
	}

	resp := internalhelpers.Ok(data)
	c.writeJSON(resp.Status, resp)
}

// GetById retorna el detalle de una oferta sin exigir tutor_id.
func (c *OfertaController) GetById() {
	ofertaID, ok := c.parseOfertaID()
	if !ok {
		return
	}

	data, err := internalservices.GetOfertaDetalle(c.Ctx.Request.Context(), int64(ofertaID))
	if err != nil {
		c.respondError(err, "error consultando oferta")
		return
	}

	resp := internalhelpers.Ok(data)
	c.writeJSON(resp.Status, resp)
}

func (c *OfertaController) requireTutor() (int, bool) {
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
func (c *OfertaController) parseOfertaID() (int, bool) {
	raw := strings.TrimSpace(c.Ctx.Input.Param(":id"))
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		c.respondError(helpers.NewAppError(http.StatusBadRequest, "id inválido", err), "id inválido")
		return 0, false
	}
	return id, true
}

func (c *OfertaController) respondError(err error, fallback string) {
	appErr := helpers.AsAppError(err, fallback)
	resp := internalhelpers.Fail(appErr.Status, appErr.Message)
	c.writeJSON(resp.Status, resp)
}

func (c *OfertaController) writeJSON(status int, payload internaldto.APIResponseDTO) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	_ = c.ServeJSON()
}

func estadoDet(code string) map[string]string {
	c := strings.ToUpper(strings.TrimSpace(code))
	nombre := c
	if par, err := internalhelpers.GetParametroByCodeNoCache(c); err == nil && strings.TrimSpace(par.Nombre) != "" {
		nombre = par.Nombre
	}
	return map[string]string{"code": c, "nombre": nombre}
}
