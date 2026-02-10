package controllers

import (
	"net/http"
	"strings"

	rootcontrollers "github.com/udistrital/pasantia_mid/controllers"
	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/models"
	"github.com/udistrital/pasantia_mid/services"
)

type TutoresController struct {
	rootcontrollers.BaseController
}

type tutorEstadoReq struct {
	NumeroDocumento string `json:"numero_documento"`
}

type upsertTutorEmpresaIn struct {
	NumeroDocumento string `json:"numero_documento"`
	Empresa         struct {
		NITSinDV    string `json:"nit_sin_dv"`
		RazonSocial string `json:"razon_social"`
	} `json:"empresa"`
}

type tutorEstadoResp struct {
	TutorID      int  `json:"tutor_id"`
	EmpresaID    *int `json:"empresa_id"`
	NeedsEmpresa bool `json:"needs_empresa"`
}

// PostEstado indica si un tutor requiere empresa asociada.
func (c *TutoresController) PostEstado() {
	var body tutorEstadoReq
	if err := c.ParseJSONBody(&body); err != nil {
		c.RespondError(err)
		return
	}

	numero := strings.TrimSpace(body.NumeroDocumento)
	if numero == "" {
		c.RespondError(helpers.NewAppError(http.StatusBadRequest, "numero_documento requerido", nil))
		return
	}

	tutorID, err := services.FindTerceroIDByDocumento(numero)
	if err != nil {
		c.RespondError(err)
		return
	}
	if tutorID == 0 {
		c.RespondError(helpers.NewAppError(http.StatusNotFound, "Tutor no registrado en Terceros", nil))
		return
	}

	empresaID, found, err := services.GetTutorEmpresaActiva(tutorID)
	if err != nil {
		c.RespondError(err)
		return
	}
	if !found {
		resp := tutorEstadoResp{TutorID: tutorID, NeedsEmpresa: true}
		c.RespondSuccess(http.StatusOK, "OK", resp)
		return
	}

	resp := tutorEstadoResp{
		TutorID:      tutorID,
		EmpresaID:    &empresaID,
		NeedsEmpresa: false,
	}
	c.RespondSuccess(http.StatusOK, "OK", resp)
}

// PostUpsertEmpresa vincula (o crea) la empresa activa del tutor.
func (c *TutoresController) PostUpsertEmpresa() {
	var body upsertTutorEmpresaIn
	if err := c.ParseJSONBody(&body); err != nil {
		c.RespondError(err)
		return
	}

	numero := strings.TrimSpace(body.NumeroDocumento)
	if numero == "" {
		c.RespondError(helpers.NewAppError(http.StatusBadRequest, "numero_documento requerido", nil))
		return
	}

	tutorID, err := services.FindTerceroIDByDocumento(numero)
	if err != nil {
		c.RespondError(err)
		return
	}
	if tutorID == 0 {
		c.RespondError(helpers.NewAppError(http.StatusNotFound, "Tutor no registrado en Terceros", nil))
		return
	}

	nit := strings.TrimSpace(body.Empresa.NITSinDV)
	if nit == "" {
		c.RespondError(helpers.NewAppError(http.StatusBadRequest, "nit_sin_dv requerido", nil))
		return
	}
	if strings.TrimSpace(body.Empresa.RazonSocial) == "" {
		c.RespondError(helpers.NewAppError(http.StatusBadRequest, "razon_social requerida", nil))
		return
	}

	empresaID, found, err := services.FindEmpresaByNIT(nit)
	if err != nil {
		c.RespondError(err)
		return
	}
	if !found {
		empresaID, err = services.CreateEmpresa(models.EmpresaInDTO{
			NITSinDV:            nit,
			RazonSocial:         body.Empresa.RazonSocial,
			TipoDocumentoId:     0,
			TipoContribuyenteId: 0,
		})
		if err != nil {
			c.RespondError(err)
			return
		}
	}

	if err := services.UpsertTutorEmpresaActiva(tutorID, empresaID); err != nil {
		c.RespondError(err)
		return
	}

	resp := tutorEstadoResp{
		TutorID:      tutorID,
		EmpresaID:    &empresaID,
		NeedsEmpresa: false,
	}
	c.RespondSuccess(http.StatusOK, "OK", resp)
}
