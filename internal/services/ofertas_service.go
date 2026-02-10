package services

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/internal/clients"
	internaldto "github.com/udistrital/pasantia_mid/internal/dto"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	"github.com/udistrital/pasantia_mid/models"
	rootservices "github.com/udistrital/pasantia_mid/services"

	beegocontext "github.com/beego/beego/v2/server/web/context"
)

const (
	// OfertaEstadoAbierta representa ofertas en estado inicial/abiertas.
	OfertaEstadoAbierta = models.OfertaEstadoCreada
	// OfertaEstadoEnCurso representa ofertas en ejecución.
	OfertaEstadoEnCurso = models.OfertaEstadoEnCurso
	// OfertaEstadoCancelada representa ofertas canceladas.
	OfertaEstadoCancelada = models.OfertaEstadoCancelada
	// OfertaEstadoPausada representa ofertas pausadas.
	OfertaEstadoPausada = models.OfertaEstadoPausada
	// OfertaEstadoFinalizada corresponde al estado de oferta finalizada en parámetros.
	OfertaEstadoFinalizada = "OPFIN_CTR"
)

// CrearOfertaReq encapsula el payload necesario para crear una oferta junto a proyectos curriculares.
type CrearOfertaReq struct {
	Oferta struct {
		Titulo      string `json:"titulo"`
		Descripcion string `json:"descripcion"`
	} `json:"oferta"`
	ProyectosCurriculares []int `json:"proyectos_curriculares"`
}

// OfertaConPCs consolida la información de la oferta creada con sus proyectos curriculares.
type crudOfertaCreateResponse struct {
	Id               int    `json:"Id"`
	FechaPublicacion string `json:"FechaPublicacion"`
	Titulo           string `json:"Titulo"`
	Descripcion      string `json:"Descripcion"`
	Estado           string `json:"Estado"`
	EmpresaId        int    `json:"EmpresaId"`
	TutorExternoID   int    `json:"tutor_externo_id"`
	//Modalidad        string `json:"Modalidad"`
}

// CrearOfertaConPCs crea una oferta y asocia proyectos curriculares en una sola transacción lógica.
func CrearOfertaConPCs(ctx context.Context, tutorID int, req CrearOfertaReq) (*internaldto.OfertaCreateResp, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	titulo := strings.TrimSpace(req.Oferta.Titulo)
	if titulo == "" {
		return nil, helpers.NewAppError(http.StatusBadRequest, "titulo requerido", nil)
	}

	if tutorID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "tutor_id requerido", nil)
	}

	empresaID, found, err := rootservices.GetTutorEmpresaActiva(tutorID)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando empresa del tutor")
	}
	if !found {
		return nil, helpers.NewAppError(http.StatusConflict, "Debes asociar una empresa antes de crear ofertas.", nil)
	}
	if empresaID <= 0 {
		return nil, helpers.NewAppError(http.StatusBadRequest, "empresa_id no disponible para el tutor", nil)
	}

	estado := models.OfertaEstadoCreada

	proyectos, err := normalizeProyectos(req.ProyectosCurriculares)
	if err != nil {
		return nil, err
	}
	if proyectos == nil {
		proyectos = []int{}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	created, err := crearOfertaCRUD(
		titulo,
		strings.TrimSpace(req.Oferta.Descripcion),
		empresaID,
		tutorID,
		"",
		estado,
	)
	if err != nil {
		return nil, err
	}

	if len(proyectos) > 0 {
		if err := asociarProyectosOferta(created.Id, proyectos); err != nil {
			_ = eliminarOfertaCRUD(created.Id)
			return nil, helpers.NewAppError(http.StatusInternalServerError, "No fue posible asociar PCs; oferta revertida", err)
		}
	}

	var fechaPtr *time.Time
	if ts := strings.TrimSpace(created.FechaPublicacion); ts != "" {
		if parsed := parseCastorDate(ts); !parsed.IsZero() {
			fecha := parsed
			fechaPtr = &fecha
		}
	}

	return &internaldto.OfertaCreateResp{
		ID:                    created.Id,
		FechaPublicacion:      fechaPtr,
		Titulo:                titulo,
		Descripcion:           strings.TrimSpace(req.Oferta.Descripcion),
		EmpresaTerceroID:      empresaID,
		Modalidad:             "",
		Estado:                created.Estado,
		TutorExternoID:        tutorID,
		ProyectosCurriculares: proyectos,
	}, nil
}

func normalizeEstado(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return models.OfertaEstadoCreada, nil
	}
	upper := strings.ToUpper(strings.TrimSpace(raw))
	switch upper {
	case models.OfertaEstadoCreada, models.OfertaEstadoCancelada, models.OfertaEstadoEnCurso, OfertaEstadoPausada, OfertaEstadoFinalizada:
		return upper, nil
	case "CREADA", "ABIERTA":
		return models.OfertaEstadoCreada, nil
	case "CANCELADA":
		return models.OfertaEstadoCancelada, nil
	case "EN_CURSO":
		return models.OfertaEstadoEnCurso, nil
	case "PAUSADA":
		return OfertaEstadoPausada, nil
	case "FINALIZADA":
		return OfertaEstadoFinalizada, nil
	default:
		return "", helpers.NewAppError(http.StatusBadRequest, "estado no soportado", nil)
	}
}

func normalizeProyectos(ids []int) ([]int, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	seen := make(map[int]struct{}, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, helpers.NewAppError(http.StatusBadRequest, "proyectos_curriculares deben ser mayores a 0", nil)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

func crearOfertaCRUD(titulo, descripcion string, empresaID, tutorExternoID int, modalidad, estado string) (crudOfertaCreateResponse, error) {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia")
	fmt.Println("Endpoint crear Oferta Pasantía", endpoint)

	payload := map[string]interface{}{
		"Titulo":      titulo,
		"Descripcion": descripcion,
		"Estado":      estado,
		"EmpresaId":   empresaID,
	}
	payload["TutorExternoId"] = tutorExternoID
	if modalidad != "" {
		payload["Modalidad"] = modalidad
	}

	var resp crudOfertaCreateResponse
	fmt.Println("Payload crear Oferta Pasantía", payload)
	if err := helpers.DoJSON("POST", endpoint, payload, &resp, cfg.RequestTimeout); err != nil {
		return resp, helpers.AsAppError(err, "error creando oferta")
	}
	if resp.Id == 0 {
		return resp, helpers.NewAppError(http.StatusInternalServerError, "respuesta inválida al crear oferta", nil)
	}
	if strings.TrimSpace(resp.Estado) == "" {
		resp.Estado = estado
	}
	return resp, nil
}

func asociarProyectosOferta(ofertaID int, proyectos []int) error {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia", strconv.Itoa(ofertaID), "carreras")

	payload := map[string][]int64{
		"proyecto_curricular_ids": make([]int64, len(proyectos)),
	}
	for i, pc := range proyectos {
		payload["proyecto_curricular_ids"][i] = int64(pc)
	}

	var resp map[string]interface{}
	if err := helpers.DoJSON("POST", endpoint, payload, &resp, cfg.RequestTimeout); err != nil {
		return helpers.AsAppError(err, "error asociando proyectos curriculares")
	}
	return nil
}

func eliminarOfertaCRUD(ofertaID int) error {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia", strconv.Itoa(ofertaID))
	return helpers.DoJSON("DELETE", endpoint, nil, nil, cfg.RequestTimeout)
}

func parseCastorDate(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed
	}
	return time.Time{}
}

// ListarOfertas retorna las ofertas asociadas a un tutor filtradas por estado.
func ListarOfertas(ctx *beegocontext.Context, tutorID int, estado string) (map[string]interface{}, error) {
	_ = ctx
	filters := map[string]string{
		"tutor_externo_id": strconv.Itoa(tutorID),
		"estado":           estado,
		"limit":            "0",
	}

	ofertas, err := rootservices.ListOfertas(filters)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando ofertas")
	}

	items := make([]map[string]interface{}, 0, len(ofertas))
	for _, oferta := range ofertas {
		items = append(items, mapOferta(oferta))
	}

	return map[string]interface{}{
		"items":  items,
		"total":  len(items),
		"estado": strings.ToUpper(strings.TrimSpace(estado)),
	}, nil
}

// ListarOfertasCatalogo lista ofertas aplicando filtros y paginación.
func ListarOfertasCatalogo(
	ctx *beegocontext.Context,
	estadosCSV string,
	tutorID, pcID int,
	estudianteID int,
	q string,
	page, size int,
	sortField, order string,
	excludePostuladas bool,
) (map[string]interface{}, error) {
	_ = ctx
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	baseFilters := map[string]string{
		"limit": "0",
	}
	if tutorID > 0 {
		baseFilters["tutor_externo_id"] = strconv.Itoa(tutorID)
	}
	if trimmed := strings.TrimSpace(q); trimmed != "" {
		baseFilters["query"] = fmt.Sprintf("Titulo__icontains:%s", trimmed)
	}
	if trimmed := strings.TrimSpace(sortField); trimmed != "" {
		baseFilters["sortby"] = trimmed
	}
	if trimmed := strings.TrimSpace(order); trimmed != "" {
		baseFilters["order"] = trimmed
	}

	states := parseEstados(estadosCSV)

	stdCtx := context.Background()
	if ctx != nil && ctx.Request != nil {
		stdCtx = ctx.Request.Context()
	}
	if pcID <= 0 && estudianteID > 0 {
		if perfil, err := clients.CastorCRUD().GetPerfilByTerceroID(stdCtx, estudianteID); err == nil && perfil != nil && perfil.ProyectoCurricularId > 0 {
			pcID = perfil.ProyectoCurricularId
		}
	}
	postuladas := map[int64]struct{}{}
	if excludePostuladas && estudianteID > 0 {
		if list, err := clients.CastorCRUD().ListPostulaciones(stdCtx, map[string]string{
			"EstudianteId": fmt.Sprint(estudianteID),
		}); err == nil {
			for _, p := range list {
				if p.OfertaId > 0 {
					postuladas[p.OfertaId] = struct{}{}
				}
			}
		}
	}

	cloneFilters := func(extra map[string]string) map[string]string {
		m := make(map[string]string, len(baseFilters)+len(extra))
		for k, v := range baseFilters {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	aggregated := make([]models.Oferta, 0)
	seen := make(map[int64]struct{})

	fetch := func(f map[string]string) error {
		result, err := rootservices.ListOfertas(f)
		if err != nil {
			return err
		}
		for _, of := range result {
			if _, ok := seen[of.Id]; ok {
				continue
			}
			seen[of.Id] = struct{}{}
			aggregated = append(aggregated, of)
		}
		return nil
	}

	if len(states) == 0 {
		if err := fetch(cloneFilters(nil)); err != nil {
			return nil, helpers.AsAppError(err, "error consultando ofertas")
		}
	} else if len(states) == 1 {
		if err := fetch(cloneFilters(map[string]string{"estado": states[0]})); err != nil {
			return nil, helpers.AsAppError(err, "error consultando ofertas")
		}
	} else {
		for _, st := range states {
			if err := fetch(cloneFilters(map[string]string{"estado": st})); err != nil {
				return nil, helpers.AsAppError(err, "error consultando ofertas")
			}
		}
	}

	items := make([]map[string]interface{}, 0, len(aggregated))
	for _, oferta := range aggregated {
		m := mapOferta(oferta)
		pcIDs, err := getPCIDsByOferta(int(oferta.Id))
		if err != nil {
			pcIDs = []int{}
		}

		include := true
		if pcID > 0 {
			if len(pcIDs) > 0 {
				include = containsInt(pcIDs, pcID)
			} else {
				include = true
			}
		}
		if !include {
			continue
		}

		m["proyecto_curricular_ids"] = pcIDs
		if excludePostuladas && estudianteID > 0 {
			if _, ok := postuladas[oferta.Id]; ok {
				continue
			}
		}

		items = append(items, m)
	}

	total := len(items)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}

	paged := items[start:end]

	return map[string]interface{}{
		"items": paged,
		"total": total,
		"page":  page,
		"size":  size,
	}, nil
}

func parseEstados(csv string) []string {
	trimmed := strings.TrimSpace(csv)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		code := strings.ToUpper(strings.TrimSpace(part))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func containsInt(list []int, target int) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

// CambiarEstadoOferta ajusta el estado de la oferta validando ownership.
func CambiarEstadoOferta(ctx *beegocontext.Context, tutorID, ofertaID int, destino string) (map[string]interface{}, error) {
	_ = ctx
	current, err := rootservices.GetOferta(int64(ofertaID))
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando oferta")
	}
	if current == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "oferta no encontrada", nil)
	}
	if int(current.TutorExternoId) != tutorID {
		return nil, helpers.NewAppError(http.StatusForbidden, "no autorizado para gestionar esta oferta", nil)
	}

	normalized := normalizeDestino(destino)
	if strings.EqualFold(strings.TrimSpace(current.Estado), strings.TrimSpace(normalized)) {
		return mapOferta(*current), nil
	}
	var updated *models.Oferta

	switch normalized {
	case OfertaEstadoCancelada:
		updated, err = rootservices.ChangeOfertaEstado(int64(ofertaID), OfertaEstadoCancelada)
	case OfertaEstadoEnCurso:
		updated, err = rootservices.ChangeOfertaEstado(int64(ofertaID), OfertaEstadoEnCurso)
	case OfertaEstadoPausada:
		updated, err = rootservices.ChangeOfertaEstado(int64(ofertaID), OfertaEstadoPausada)
	case models.OfertaEstadoCreada:
		updated, err = rootservices.ChangeOfertaEstado(int64(ofertaID), models.OfertaEstadoCreada)
	case OfertaEstadoFinalizada:
		updated, err = rootservices.ChangeOfertaEstado(int64(ofertaID), OfertaEstadoFinalizada)
	default:
		return nil, helpers.NewAppError(http.StatusBadRequest, "estado destino no soportado", nil)
	}

	if err != nil {
		return nil, helpers.AsAppError(err, "error actualizando oferta")
	}

	return mapOferta(*updated), nil
}

func normalizeDestino(raw string) string {
	upper := strings.ToUpper(strings.TrimSpace(raw))
	switch upper {
	case "PAUSAR", "PAUSADA":
		return OfertaEstadoPausada
	case "REACTIVAR", "ABIERTA", "CREADA":
		return models.OfertaEstadoCreada
	case "FINALIZAR", "FINALIZADA":
		return OfertaEstadoFinalizada
	default:
		return upper
	}
}

func mapOferta(oferta models.Oferta) map[string]interface{} {
	code := strings.TrimSpace(oferta.Estado)
	nombre := code
	if code != "" {
		if par, err := internalhelpers.GetParametroByCodeNoCache(code); err == nil {
			if n := strings.TrimSpace(par.Nombre); n != "" {
				nombre = n
			}
		}
	}

	return map[string]interface{}{
		"id":          oferta.Id,
		"titulo":      strings.TrimSpace(oferta.Titulo),
		"descripcion": strings.TrimSpace(oferta.Descripcion),
		"estado":      code,
		"estado_det": map[string]string{
			"code":   code,
			"nombre": nombre,
		},
		"empresa_id":              oferta.EmpresaId,
		"tutor_externo_id":        oferta.TutorExternoId,
		"fecha_publicacion":       oferta.FechaPublicacion,
		"proyecto_curricular_ids": oferta.ProyectoCurricularIds,
	}
}

// GetOfertaDetalle retorna el detalle de una oferta por id.
func GetOfertaDetalle(ctx context.Context, ofertaID int64) (map[string]interface{}, error) {
	oferta, err := rootservices.GetOferta(ofertaID)
	if err != nil {
		return nil, helpers.AsAppError(err, "error consultando oferta")
	}
	if oferta == nil {
		return nil, helpers.NewAppError(http.StatusNotFound, "oferta no encontrada", nil)
	}
	return mapOferta(*oferta), nil
}
