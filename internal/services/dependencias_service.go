package services

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/udistrital/pasantia_mid/helpers"
	rootservices "github.com/udistrital/pasantia_mid/services"

	"github.com/beego/beego/v2/server/web/context"
)

type homologacionResp struct {
	ID_Oikos       int    `xml:"id_oikos"`
	CodigoProyecto int    `xml:"codigo_proyecto"`
	ProyectoSnies  string `xml:"proyecto_snies"`
}

func HomologarAcademicaToOikos(ctx *context.Context, codigoAcademica int) (idOikos int, nombre string, err error) {
	cfg := rootservices.GetConfig()
	base := strings.TrimRight(cfg.DependenciasAPIBaseURL, "/")
	if base == "" {
		return 0, "", fmt.Errorf("DependenciasAPIBaseURL no configurado")
	}
	url := fmt.Sprintf("%s/proyecto_curricular_cod_proyecto/%d", base, codigoAcademica)

	xmlStr, err := getText(ctx, url)
	if err != nil {
		return 0, "", err
	}

	var h homologacionResp
	if err := xml.Unmarshal([]byte(xmlStr), &h); err != nil {
		return 0, "", fmt.Errorf("parse xml homologacion: %w", err)
	}

	nombre = strings.TrimSpace(h.ProyectoSnies)
	if h.ID_Oikos <= 0 {
		return 0, nombre, fmt.Errorf("homologación sin id_oikos para codigo=%d", codigoAcademica)
	}
	return h.ID_Oikos, nombre, nil
}

// ResolverProyectoCurricularPorCodigo resuelve la homologación Académica -> Oikos.
func ResolverProyectoCurricularPorCodigo(ctx *context.Context, codigo int) (map[string]interface{}, error) {
	if codigo <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "codigo inválido", nil)
	}

	idOikos, nombre, err := HomologarAcademicaToOikos(ctx, codigo)
	if err != nil {
		return nil, helpers.NewAppError(http.StatusBadGateway, "error homologando proyecto curricular", err)
	}
	if idOikos <= 0 {
		return nil, helpers.NewAppError(http.StatusBadGateway, "homologación sin id_oikos", nil)
	}

	// Best-effort: si nombre viene vacío, intenta con Oikos directo.
	if strings.TrimSpace(nombre) == "" {
		if detalle, err := rootservices.GetProyectoCurricular(idOikos); err == nil && detalle != nil {
			n := strings.TrimSpace(detalle.Nombre)
			if n != "" {
				nombre = n
			}
		}
	}

	return map[string]interface{}{
		"codigo":   codigo,
		"id_oikos": idOikos,
		"nombre":   strings.TrimSpace(nombre),
	}, nil
}

func getText(ctx *context.Context, url string) (string, error) {

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if ctx != nil && ctx.Request != nil {
		req = req.WithContext(ctx.Request.Context())
	}
	req.Header.Set("Accept", "application/xml")

	// Propagar correlation/auth entrante (igual que GetJSON)
	if ctx != nil {
		if corr := ctx.Input.Header("X-Correlation-Id"); corr != "" {
			req.Header.Set("X-Correlation-Id", corr)
		}
		if auth := ctx.Input.Header("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}
	}

	resp, err := http.DefaultClient.Do(req) // si no existe, usa http.DefaultClient.Do
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GET %s -> %d: %s", url, resp.StatusCode, string(b))
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
