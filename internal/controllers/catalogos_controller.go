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
	rootservices "github.com/udistrital/pasantia_mid/services"
)

// CatalogosController expone catálogos públicos sin autenticación.
type CatalogosController struct {
	rootcontrollers.BaseController
}

// GetFacultades lista las facultades disponibles.
func (c *CatalogosController) GetFacultades() {
	q := strings.TrimSpace(c.GetString("q"))
	page := strings.TrimSpace(c.GetString("page"))
	size := strings.TrimSpace(c.GetString("size"))

	result, err := internalservices.ListarFacultades(c.Ctx, q, page, size)
	if err != nil {
		resp := c.buildError(err, "error consultando facultades")
		c.writeJSON(resp.Status, resp)
		return
	}

	resp := internalhelpers.Ok(result)
	c.writeJSON(resp.Status, resp)
}

// GetPCPorFacultad lista los proyectos curriculares asociados a una facultad.
func (c *CatalogosController) GetPCPorFacultad() {
	rawID := strings.TrimSpace(c.Ctx.Input.Param(":id"))
	facultadID, err := strconv.Atoi(rawID)
	if err != nil || facultadID <= 0 {
		resp := internalhelpers.Fail(http.StatusBadRequest, "facultad_id inválido")
		c.writeJSON(resp.Status, resp)
		return
	}

	q := strings.TrimSpace(c.GetString("q"))
	page := strings.TrimSpace(c.GetString("page"))
	size := strings.TrimSpace(c.GetString("size"))

	result, err := internalservices.ListarPCPorFacultad(c.Ctx, facultadID, q, page, size)
	fmt.Println("Consulta por facultad", result)
	if err != nil {
		resp := c.buildError(err, "error consultando proyectos curriculares")
		c.writeJSON(resp.Status, resp)
		return
	}

	resp := internalhelpers.Ok(result)
	c.writeJSON(resp.Status, resp)
}

// GetProyectosCurriculares lista los proyectos curriculares, filtrando opcionalmente por facultad.
func (c *CatalogosController) GetProyectosCurriculares() {
	q := strings.TrimSpace(c.GetString("q"))
	page := strings.TrimSpace(c.GetString("page"))
	size := strings.TrimSpace(c.GetString("size"))
	facultadRaw := strings.TrimSpace(c.GetString("facultad_id"))

	if facultadRaw != "" {
		facultadID, err := strconv.Atoi(facultadRaw)
		if err != nil || facultadID <= 0 {
			resp := internalhelpers.Fail(http.StatusBadRequest, "facultad_id inválido")
			c.writeJSON(resp.Status, resp)
			return
		}

		result, err := internalservices.ListarPCPorFacultad(c.Ctx, facultadID, q, page, size)
		if err != nil {
			resp := c.buildError(err, "error consultando proyectos curriculares")
			c.writeJSON(resp.Status, resp)
			return
		}

		resp := internalhelpers.Ok(map[string]interface{}{
			"items": result,
			"total": len(result),
		})
		c.writeJSON(resp.Status, resp)
		return
	}

	result, err := internalservices.ListarProyectosCurriculares(c.Ctx, q, page, size)
	if err != nil {
		resp := c.buildError(err, "error consultando proyectos curriculares")
		c.writeJSON(resp.Status, resp)
		return
	}

	resp := internalhelpers.Ok(map[string]interface{}{
		"items": result,
		"total": len(result),
	})
	c.writeJSON(resp.Status, resp)
}

// GetProyectoCurricular obtiene la información básica (id/nombre) de un proyecto curricular por Id.
func (c *CatalogosController) GetProyectoCurricular() {
	rawID := strings.TrimSpace(c.Ctx.Input.Param(":id"))
	proyectoID, err := strconv.Atoi(rawID)
	if err != nil || proyectoID <= 0 {
		resp := internalhelpers.Fail(http.StatusBadRequest, "proyecto_curricular_id inválido")
		c.writeJSON(resp.Status, resp)
		return
	}

	nombre, err := internalservices.ObtenerNombreProyectoCurricular(c.Ctx, proyectoID)
	if err != nil {
		resp := c.buildError(err, "error consultando proyecto curricular")
		c.writeJSON(resp.Status, resp)
		return
	}
	if nombre == "" {
		resp := internalhelpers.Fail(http.StatusNotFound, "proyecto curricular no encontrado")
		c.writeJSON(resp.Status, resp)
		return
	}

	resp := internalhelpers.Ok(map[string]interface{}{
		"id":     proyectoID,
		"nombre": nombre,
	})
	c.writeJSON(resp.Status, resp)
}

// GetProyectoCurricularPorCodigo obtiene el proyecto curricular a partir del código Académica.
// curl "http://localhost:8080/v1/catalogos/proyecto-curricular?codigo=578"
func (c *CatalogosController) GetProyectoCurricularPorCodigo() {
	raw := strings.TrimSpace(c.GetString("codigo"))
	codigo, err := strconv.Atoi(raw)
	if err != nil || codigo <= 0 {
		resp := internalhelpers.Fail(http.StatusBadRequest, "codigo inválido")
		c.writeJSON(resp.Status, resp)
		return
	}

	result, err := internalservices.ResolverProyectoCurricularPorCodigo(c.Ctx, codigo)
	if err != nil {
		resp := c.buildError(err, "error consultando proyecto curricular")
		c.writeJSON(resp.Status, resp)
		return
	}

	resp := internalhelpers.Ok(result)
	c.writeJSON(resp.Status, resp)
}

// GetEstadosOferta retorna el catálogo de estados de oferta.
func (c *CatalogosController) GetEstadosOferta() {
	catalogo, err := rootservices.GetEstadosCatalogo()
	if err != nil {
		resp := c.buildError(err, "error consultando estados de oferta")
		c.writeJSON(resp.Status, resp)
		return
	}

	data := catalogo.Oferta
	if data == nil {
		data = map[string]int{}
	}

	resp := internalhelpers.Ok(data)
	c.writeJSON(resp.Status, resp)
}

func (c *CatalogosController) buildError(err error, fallback string) internaldto.APIResponseDTO {
	appErr := helpers.AsAppError(err, fallback)
	return internalhelpers.Fail(appErr.Status, appErr.Message)
}

func (c *CatalogosController) writeJSON(status int, payload internaldto.APIResponseDTO) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	_ = c.ServeJSON()
}
