package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	stdctx "context"

	midhelpers "github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/internal/dto"
	"github.com/udistrital/pasantia_mid/internal/helpers"
	cfgsvc "github.com/udistrital/pasantia_mid/services"

	"github.com/beego/beego/v2/server/web/context"
)

func tercerosBase() (string, error) {
	base := strings.TrimSpace(cfgsvc.GetConfig().TercerosBaseURL)
	source := "config"
	if base == "" {
		base = helpers.Env("TERCEROS_URL", "")
		source = "env"
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return "", fmt.Errorf("TERCEROS_URL/TERCEROS_BASE_URL no configurada")
	}
	base = strings.TrimRight(base, "/")
	if helpers.Env("DEBUG_TERCEROS_CONFIG", "") != "" {
		fmt.Println("terceros base url source:", source, "value:", base)
	}
	return base, nil
}

func pathTercero() string { return helpers.Env("TERCEROS_TERCERO_PATH", "/tercero") }
func pathDatosIdentificacion() string {
	return helpers.Env("TERCEROS_DIDENT_PATH", "/datos-identificacion")
}
func pathVinculacion() string { return helpers.Env("TERCEROS_VINC_PATH", "/vinculaciones") }

type UpTercero struct {
	Id                  int        `json:"Id,omitempty"`
	NombreCompleto      string     `json:"NombreCompleto"`
	PrimerNombre        string     `json:"PrimerNombre,omitempty"`
	SegundoNombre       string     `json:"SegundoNombre,omitempty"`
	PrimerApellido      string     `json:"PrimerApellido,omitempty"`
	SegundoApellido     string     `json:"SegundoApellido,omitempty"`
	FechaNacimiento     *time.Time `json:"FechaNacimiento,omitempty"`
	Activo              bool       `json:"Activo"`
	TipoContribuyenteID int        `json:"TipoContribuyenteId"`
	FechaCreacion       *time.Time `json:"FechaCreacion,omitempty"`
	FechaModificacion   *time.Time `json:"FechaModificacion,omitempty"`
	UsuarioWSO2         string     `json:"UsuarioWSO2,omitempty"`
}

type UpDatosIdentificacion struct {
	Id                int        `json:"Id,omitempty"`
	TipoDocumentoID   int        `json:"TipoDocumentoId"`
	TerceroID         int        `json:"TerceroId"`
	Numero            string     `json:"Numero"`
	Activo            bool       `json:"Activo"`
	FechaCreacion     *time.Time `json:"FechaCreacion,omitempty"`
	FechaModificacion *time.Time `json:"FechaModificacion,omitempty"`
}

type UpVinculacion struct {
	Id                   int        `json:"Id,omitempty"`
	TerceroPrincipalID   int        `json:"TerceroPrincipalId"`
	TerceroRelacionadoID int        `json:"TerceroRelacionadoId"`
	Activo               bool       `json:"Activo"`
	FechaCreacion        *time.Time `json:"FechaCreacion,omitempty"`
	FechaModificacion    *time.Time `json:"FechaModificacion,omitempty"`
}

func postJSON(ctx *context.Context, fullURL string, in any, out any) error {
	payload, _ := json.Marshal(in)
	req, err := helpers.NewJSONRequest(ctx, http.MethodPost, fullURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	return helpers.DoJSON(req, out)
}

func getJSON(ctx *context.Context, fullURL string, out any) error {
	req, err := helpers.NewJSONRequest(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}
	return helpers.DoJSON(req, out)
}

type dIdentRow struct {
	Id        int    `json:"Id"`
	TerceroID int    `json:"TerceroId"`
	Numero    string `json:"Numero"`
}

func buscarEmpresaPorNIT(ctx *context.Context, tipoDocumentoID int, nit string) (int, error) {
	base, err := tercerosBase()
	if err != nil {
		return 0, err
	}
	u, _ := url.Parse(base + pathDatosIdentificacion())
	q := u.Query()
	q.Set("TipoDocumentoId", strconv.Itoa(tipoDocumentoID))
	q.Set("Numero", strings.TrimSpace(nit))
	u.RawQuery = q.Encode()

	var rows []dIdentRow
	if err := getJSON(ctx, u.String(), &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].TerceroID, nil
}

func crearTercero(ctx *context.Context, payload UpTercero) (int, error) {
	base, err := tercerosBase()
	if err != nil {
		return 0, err
	}
	var out UpTercero
	if err := postJSON(ctx, base+pathTercero(), payload, &out); err != nil {
		return 0, err
	}
	if out.Id == 0 {
		return 0, fmt.Errorf("tercero creado sin Id")
	}
	return out.Id, nil
}

func crearDatosIdentificacion(ctx *context.Context, payload UpDatosIdentificacion) (int, error) {
	base, err := tercerosBase()
	if err != nil {
		return 0, err
	}
	var out UpDatosIdentificacion
	if err := postJSON(ctx, base+pathDatosIdentificacion(), payload, &out); err != nil {
		return 0, err
	}
	if out.Id == 0 {
		return 0, fmt.Errorf("datos_identificacion creado sin Id")
	}
	return out.Id, nil
}

func crearVinculacion(ctx *context.Context, payload UpVinculacion) (int, error) {
	base, err := tercerosBase()
	if err != nil {
		return 0, err
	}
	var out UpVinculacion
	if err := postJSON(ctx, base+pathVinculacion(), payload, &out); err != nil {
		return 0, err
	}
	if out.Id == 0 {
		return 0, fmt.Errorf("vinculación creada sin Id")
	}
	return out.Id, nil
}

func RegistrarTutorExterno(ctx *context.Context, req dto.TutorExternoRegistroReq) (*dto.TutorExternoRegistroResp, error) {
	now := timePtr(time.Now())

	empresaID, err := buscarEmpresaPorNIT(ctx, req.EmpresaTipoDocumento, req.EmpresaNIT)
	if err != nil {
		return nil, fmt.Errorf("error buscando empresa por NIT: %w", err)
	}

	if empresaID == 0 || req.ForzarCrearEmpresa {
		empresaPayload := UpTercero{
			NombreCompleto:      req.EmpresaNombreCompleto,
			Activo:              true,
			TipoContribuyenteID: req.TipoContribuyenteID,
			FechaCreacion:       now,
			FechaModificacion:   now,
		}
		empresaID, err = crearTercero(ctx, empresaPayload)
		if err != nil {
			return nil, fmt.Errorf("no fue posible crear empresa: %w", err)
		}
		_, err = crearDatosIdentificacion(ctx, UpDatosIdentificacion{
			TipoDocumentoID:   req.EmpresaTipoDocumento,
			TerceroID:         empresaID,
			Numero:            req.EmpresaNIT,
			Activo:            true,
			FechaCreacion:     now,
			FechaModificacion: now,
		})
		if err != nil {
			return nil, fmt.Errorf("no fue posible crear NIT de la empresa: %w", err)
		}
	}

	tutorPayload := UpTercero{
		NombreCompleto:      req.NombreCompleto,
		PrimerNombre:        req.PrimerNombre,
		SegundoNombre:       req.SegundoNombre,
		PrimerApellido:      req.PrimerApellido,
		SegundoApellido:     req.SegundoApellido,
		FechaNacimiento:     req.FechaNacimiento,
		Activo:              req.Activo,
		TipoContribuyenteID: req.TipoContribuyenteID,
		FechaCreacion:       now,
		FechaModificacion:   now,
		UsuarioWSO2:         req.UsuarioWSO2,
	}
	tutorID, err := crearTercero(ctx, tutorPayload)
	if err != nil {
		return nil, fmt.Errorf("no fue posible crear tutor externo: %w", err)
	}

	_, err = crearDatosIdentificacion(ctx, UpDatosIdentificacion{
		TipoDocumentoID:   req.TutorTipoDocumentoID,
		TerceroID:         tutorID,
		Numero:            req.TutorNumero,
		Activo:            true,
		FechaCreacion:     now,
		FechaModificacion: now,
	})
	if err != nil {
		return nil, fmt.Errorf("no fue posible crear documento del tutor: %w", err)
	}

	vincID, err := crearVinculacion(ctx, UpVinculacion{
		TerceroPrincipalID:   empresaID,
		TerceroRelacionadoID: tutorID,
		Activo:               true,
		FechaCreacion:        now,
		FechaModificacion:    now,
	})
	if err != nil {
		return nil, fmt.Errorf("no fue posible vincular empresa y tutor: %w", err)
	}

	return &dto.TutorExternoRegistroResp{
		EmpresaID:      empresaID,
		TutorExternoID: tutorID,
		VinculacionID:  vincID,
	}, nil
}

func ObtenerEmpresaPorID(ctx *context.Context, id int) (map[string]any, error) {
	base, err := tercerosBase()
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s%s/%d", base, pathTercero(), id)
	var empresa map[string]any
	if err := getJSON(ctx, endpoint, &empresa); err != nil {
		return nil, err
	}
	return empresa, nil
}

func ObtenerTutorExternoPorID(ctx *context.Context, id int) (map[string]any, error) {
	base, err := tercerosBase()
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s%s/%d", base, pathTercero(), id)
	fmt.Println("URL Terceros!!!", endpoint)
	var tutor map[string]any
	if err := getJSON(ctx, endpoint, &tutor); err != nil {
		return nil, err
	}
	return tutor, nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// ObtenerTerceroPorIDCore: usa context.Context estándar (NO beego context).
func ObtenerTerceroPorIDCore(ctx context.Context, id int) (map[string]any, error) {
	fmt.Println("LLAMADO AL TERCERO POR ID CORE")
	if id <= 0 {
		return nil, fmt.Errorf("id inválido")
	}

	cfg := cfgsvc.GetConfig()
	base := strings.TrimRight(strings.TrimSpace(cfg.TercerosBaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("TERCEROS_BASE_URL no configurada")
	}

	// base normalmente ya viene con /v1, y pathTercero() por defecto es /tercero
	endpoint := fmt.Sprintf("%s%s/%d", base, pathTercero(), id)

	var out map[string]any
	// Usa el helper del MID (el mismo patrón que ya usas en castor_crud_client.go)
	if err := midhelpers.DoJSON("GET", endpoint, nil, &out, cfg.RequestTimeout); err != nil {
		return nil, err
	}
	fmt.Println("OBTENCIÓN DE TERCERO POR ID CORE", out)
	return out, nil
}

// NombreCompletoPorIDCore: retorna NombreCompleto listo para UI/enrichment.
func NombreCompletoPorIDCore(ctx context.Context, id int) string {
	m, err := ObtenerTerceroPorIDCore(ctx, id)
	if err != nil || m == nil {
		return ""
	}

	// la respuesta de terceros es un map plano con keys tipo "NombreCompleto"
	fmt.Println("NOMBRE COMPLETO POR ID CORE", m)
	if v, ok := m["NombreCompleto"]; ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func getJSONCoreStd(ctx stdctx.Context, fullURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	token := strings.TrimSpace(cfgsvc.GetConfig().OASBearerToken)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// helpers.DoJSON aquí es el internal/helpers (firma: (*http.Request, interface{}))
	return helpers.DoJSON(req, out)
}

// ObtenerTerceroPorIDCoreStd: usa context.Context estándar (NO beego context).
func ObtenerTerceroPorIDCoreStd(ctx stdctx.Context, id int) (map[string]any, error) {
	if id <= 0 {
		return nil, fmt.Errorf("id inválido")
	}

	cfg := cfgsvc.GetConfig()
	base := strings.TrimRight(strings.TrimSpace(cfg.TercerosBaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("TERCEROS_BASE_URL no configurada")
	}

	// OJO: si cfg.TercerosBaseURL ya trae /v1, esto queda: {base}/tercero/{id}
	endpoint := fmt.Sprintf("%s%s/%d", base, pathTercero(), id)

	var out map[string]any
	if err := getJSONCoreStd(ctx, endpoint, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func NombreCompletoPorIDCoreStd(ctx stdctx.Context, id int) string {
	m, err := ObtenerTerceroPorIDCoreStd(ctx, id)
	if err != nil || m == nil {
		return ""
	}
	if v, ok := m["NombreCompleto"]; ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}
