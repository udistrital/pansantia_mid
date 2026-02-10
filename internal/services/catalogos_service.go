package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	beego "github.com/beego/beego/v2/server/web"
	webctx "github.com/beego/beego/v2/server/web/context"

	"github.com/udistrital/pasantia_mid/internal/helpers"
	rootservices "github.com/udistrital/pasantia_mid/services"
)

// ===========================
// DTOs locales (simples)
// ===========================

type OpcionDTO struct {
	Id     int    `json:"id"`
	Nombre string `json:"nombre"`
}

// Respuesta de OIKOS v2 /proyecto_curricular/*
type nodoOikos struct {
	Id       int         `json:"Id"`
	Nombre   string      `json:"Nombre"`
	Padre    int         `json:"Padre"`
	Hija     int         `json:"Hija"`
	Opciones interface{} `json:"Opciones"`
}
type respOikosProys struct {
	Body []nodoOikos `json:"Body"`
	Type string      `json:"Type"`
}

//	type tipoDependenciaV2 struct {
//		Id     int    `json:"Id"`
//		Nombre string `json:"Nombre"`
//		Activo bool   `json:"Activo"`
//	}
type tipoDependenciaV2 struct {
	Id                int    `json:"Id"`
	Nombre            string `json:"Nombre"`
	Descripcion       string `json:"Descripcion"`
	CodigoAbreviacion string `json:"CodigoAbreviacion"`
	Activo            bool   `json:"Activo"`
}

type dependenciaTipoV2 struct {
	Id                int      `json:"Id"`
	TipoDependenciaId intOrObj `json:"TipoDependenciaId"`
	DependenciaId     intOrObj `json:"DependenciaId"`
	Activo            bool     `json:"Activo"`
}
type dependenciaV2 struct {
	Id     int    `json:"Id"`
	Nombre string `json:"Nombre"`
	Activo bool   `json:"Activo"`
}

// ===========================
// Cache simple en memoria
// ===========================

var (
	cacheTTL = 30 * time.Minute

	cacheFacultades struct {
		mu        sync.RWMutex
		expiresAt time.Time
		data      []OpcionDTO
	}
)

// ===========================
// Funciones públicas (usadas por el controller)
// ===========================

type intOrObj struct {
	Id int
}

func (t *intOrObj) UnmarshalJSON(b []byte) error {
	// null
	if len(b) == 0 || string(b) == "null" {
		t.Id = 0
		return nil
	}
	// ¿objeto?
	if len(b) > 0 && b[0] == '{' {
		var tmp struct {
			Id int `json:"Id"`
		}
		if err := json.Unmarshal(b, &tmp); err != nil {
			return err
		}
		t.Id = tmp.Id
		return nil
	}
	// ¿número?
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		t.Id = n
		return nil
	}
	// ¿string con número?
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		// intenta parsear string->int si llega como texto
		if i, err2 := strconv.Atoi(strings.TrimSpace(s)); err2 == nil {
			t.Id = i
			return nil
		}
	}
	return fmt.Errorf("valor no soportado para intOrObj: %s", string(b))
}

// ListarFacultades retorna las facultades (id/nombre) con filtro y paginación opcional.
func ListarFacultades(ctx *webctx.Context, q, page, size string) ([]OpcionDTO, error) {
	// 1) Cargar catálogo (con cache)
	all, err := facultadesCatalogo(ctx)
	if err != nil {
		return nil, err
	}

	// 2) Filtrado por q (case-insensitive contiene)
	if s := strings.TrimSpace(q); s != "" {
		all = filterByNombre(all, s)
	}

	// 3) Orden + paginación
	sortByNombre(all)
	return paginate(all, page, size), nil
}

// ListarPCPorFacultad retorna los proyectos curriculares (id/nombre) de una facultad concreta.
func ListarPCPorFacultad(ctx *webctx.Context, facultadID int, q, page, size string) ([]OpcionDTO, error) {
	oikos, err := getOikosBaseURL()
	if err != nil {
		return nil, fmt.Errorf("configuración OikosService: %w", err)
	}
	url := fmt.Sprintf("http://%s/proyecto_curricular/get_all_proyectos_by_facultad_id/%d", oikos, facultadID)
	fmt.Println("URL OIKOS", url)

	var ans respOikosProys
	if err := helpers.GetJSON(ctx, url, &ans, nil); err != nil {
		return nil, err
	}
	if strings.ToLower(ans.Type) != "success" {
		return []OpcionDTO{}, nil
	}

	// Normalización: tomamos hijos de primer nivel (Opciones) cuando el Body trae la facultad como nodo.
	out := make([]OpcionDTO, 0)
	if len(ans.Body) == 1 && ans.Body[0].Id == facultadID && ans.Body[0].Opciones != nil {
		out = extraerHijosPrimerNivel(ans.Body[0].Opciones, facultadID)
	} else {
		for _, n := range ans.Body {
			if n.Id != 0 && strings.TrimSpace(n.Nombre) != "" {
				out = append(out, OpcionDTO{Id: n.Id, Nombre: n.Nombre})
			}
		}
	}

	// Filtro q + orden + paginado
	if s := strings.TrimSpace(q); s != "" {
		out = filterByNombre(out, s)
	}
	sortByNombre(out)
	return paginate(out, page, size), nil
}

// ListarProyectosCurriculares retorna todos los proyectos curriculares (de todas las facultades).
// Para eficiencia, usamos el endpoint "all-in-one" y aplanamos.
func ListarProyectosCurriculares(ctx *webctx.Context, q, page, size string) ([]OpcionDTO, error) {
	if size == "0" {
		return listarProyectosCurricularesAll(q)
	}
	body, err := obtenerProyectosPorFacultad(ctx)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return []OpcionDTO{}, nil
	}

	out := flattenProyectosPorFacultad(body)

	if s := strings.TrimSpace(q); s != "" {
		out = filterByNombre(out, s)
	}
	sortByNombre(out)
	return paginate(out, page, size), nil
}

// ObtenerNombreProyectoCurricular busca el nombre del proyecto curricular por Id reutilizando el catálogo completo.
func ObtenerNombreProyectoCurricular(ctx *webctx.Context, proyectoID int) (string, error) {
	if proyectoID <= 0 {
		return "", nil
	}

	body, err := obtenerProyectosPorFacultad(ctx)
	if err != nil {
		return "", err
	}
	if len(body) == 0 {
		return "", nil
	}

	nombre := buscarNombreProyecto(body, proyectoID)
	return strings.TrimSpace(nombre), nil
}

// ===========================
// Internos (OIKOS v2)
// ===========================

func facultadesCatalogo(ctx *webctx.Context) ([]OpcionDTO, error) {
	// cache
	cacheFacultades.mu.RLock()
	if time.Now().Before(cacheFacultades.expiresAt) && cacheFacultades.data != nil {
		defer cacheFacultades.mu.RUnlock()
		cl := make([]OpcionDTO, len(cacheFacultades.data))
		copy(cl, cacheFacultades.data)
		return cl, nil
	}
	cacheFacultades.mu.RUnlock()

	// 1) id del tipo "FACULTAD"
	idTipo, err := getTipoDependenciaId(ctx, "FACULTAD")
	if err != nil || idTipo == 0 {
		return nil, fmt.Errorf("no se encontró tipo_dependencia 'FACULTAD': %w", err)
	}
	fmt.Println("Respuesta ID Tipo Faculotad¡", idTipo)
	// 2) todas las dependencias de ese tipo (ids)
	ids, err := getDependenciaIdsPorTipo(ctx, idTipo)
	if err != nil {
		return nil, fmt.Errorf("error consultando dependencias tipo FACULTAD: %w", err)
	}
	if len(ids) == 0 {
		return []OpcionDTO{}, nil
	}
	fmt.Println("Respuesta IDs  Faculotades", ids)

	// 3) detalle (nombres)
	deps, err := getDependenciasDetalle(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("error consultando detalle de dependencias: %w", err)
	}

	out := make([]OpcionDTO, 0, len(deps))
	for _, d := range deps {
		if d.Activo && d.Id != 0 && strings.TrimSpace(d.Nombre) != "" {
			out = append(out, OpcionDTO{Id: d.Id, Nombre: d.Nombre})
		}
	}
	sortByNombre(out)

	// set cache
	cacheFacultades.mu.Lock()
	cacheFacultades.data = out
	cacheFacultades.expiresAt = time.Now().Add(cacheTTL)
	cacheFacultades.mu.Unlock()

	return out, nil
}

func getTipoDependenciaId(ctx *webctx.Context, nombre string) (int, error) {
	oikos, err := getOikosBaseURL()
	if err != nil {
		return 0, fmt.Errorf("configuración OikosService: %w", err)
	}
	// limit=1 — nombre exacto + activo
	url := fmt.Sprintf("http://%s/tipo_dependencia?query=nombre:%s,activo:true&limit=1", oikos, urlEscape(nombre))

	fmt.Println("URL TIPO DEPENDENCIA", url)
	var rows []tipoDependenciaV2

	if err := helpers.GetJSON(ctx, url, &rows, nil); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("tipo '%s' no encontrado", nombre)
	}

	return rows[0].Id, nil
}

func getDependenciaIdsPorTipo(ctx *webctx.Context, idTipo int) ([]int, error) {
	oikos, err := getOikosBaseURL()
	if err != nil {
		return nil, fmt.Errorf("configuración OikosService: %w", err)
	}
	url := fmt.Sprintf("http://%s/dependencia_tipo_dependencia?query=tipo_dependencia_id:%d,activo:true&limit=0", oikos, idTipo)

	var rows []dependenciaTipoV2
	if err := helpers.GetJSON(ctx, url, &rows, nil); err != nil {
		return nil, err
	}

	out := make([]int, 0, len(rows))
	seen := map[int]bool{}
	for _, r := range rows {
		depID := r.DependenciaId.Id
		// (opcional)  validar r.TipoDependenciaId.Id == idTipo por si el CRUD ignorara el filtro
		if r.Activo && depID != 0 && !seen[depID] {
			out = append(out, depID)
			seen[depID] = true
		}
	}
	return out, nil
}

func getDependenciasDetalle(ctx *webctx.Context, ids []int) ([]dependenciaV2, error) {
	if len(ids) == 0 {
		return []dependenciaV2{}, nil
	}
	oikos, err := getOikosBaseURL()
	if err != nil {
		return nil, fmt.Errorf("configuración OikosService: %w", err)
	}

	// Intento 1: __in
	inCSV := joinInt(ids, "|")
	urlIN := fmt.Sprintf("http://%s/dependencia?query=id__in:%s,activo:true&limit=0", oikos, inCSV)

	var items []dependenciaV2
	if err := helpers.GetJSON(ctx, urlIN, &items, nil); err == nil && len(items) > 0 {
		return items, nil
	}

	// Fallback: N requests individuales
	out := make([]dependenciaV2, 0, len(ids))
	for _, id := range ids {
		var d dependenciaV2
		url := fmt.Sprintf("http://%s/dependencia/%d", oikos, id)
		if err := helpers.GetJSON(ctx, url, &d, nil); err == nil && d.Id != 0 && strings.TrimSpace(d.Nombre) != "" {
			out = append(out, d)
		}
	}
	return out, nil
}

// ===========================
// Utilitarios locales
// ===========================

func extraerHijosPrimerNivel(opciones interface{}, padre int) []OpcionDTO {
	arr, ok := opciones.([]interface{})
	if !ok || len(arr) == 0 {
		return []OpcionDTO{}
	}
	out := make([]OpcionDTO, 0, len(arr))
	seen := map[int]bool{}

	for _, raw := range arr {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id := asInt(m["Id"])
		nombre, _ := m["Nombre"].(string)
		p := asInt(m["Padre"])
		if id == 0 || strings.TrimSpace(nombre) == "" || p != padre || seen[id] {
			continue
		}
		out = append(out, OpcionDTO{Id: id, Nombre: nombre})
		seen[id] = true
	}
	return out
}

func filterByNombre(xs []OpcionDTO, q string) []OpcionDTO {
	q = strings.ToLower(q)
	out := make([]OpcionDTO, 0, len(xs))
	for _, it := range xs {
		if strings.Contains(strings.ToLower(it.Nombre), q) {
			out = append(out, it)
		}
	}
	return out
}

func sortByNombre(xs []OpcionDTO) {
	sort.Slice(xs, func(i, j int) bool {
		return strings.Compare(xs[i].Nombre, xs[j].Nombre) < 0
	})
}

func paginate(xs []OpcionDTO, pageStr, sizeStr string) []OpcionDTO {
	if len(xs) == 0 {
		return xs
	}
	// defaults
	page := 1
	size := 50
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 && s <= 500 {
		size = s
	}
	from := (page - 1) * size
	if from >= len(xs) {
		return []OpcionDTO{}
	}
	to := from + size
	if to > len(xs) {
		to = len(xs)
	}
	return xs[from:to]
}

func asInt(v interface{}) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return 0
	}
}

func joinInt(a []int, sep string) string {
	if len(a) == 0 {
		return ""
	}
	sb := strings.Builder{}
	for i, x := range a {
		if i > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(fmt.Sprintf("%d", x))
	}
	return sb.String()
}

func getOikosBaseURL() (string, error) {
	candidates := []string{
		helpers.Env("OIKOS_BASE_URL", ""),
		helpers.Env("OIKOS_SERVICE", ""),
		helpers.Env("OIKOS_V2_BASE_URL", ""),
		helpers.Env("OIKOS_V1_BASE_URL", ""),
	}

	for _, key := range []string{"OikosService", "oikos_base_url", "oikos_v2_base_url", "oikos_v1_base_url"} {
		if val, err := beego.AppConfig.String(key); err == nil {
			candidates = append(candidates, val)
		}
	}

	for _, raw := range candidates {
		if normalized := normalizeOikosHost(raw); normalized != "" {
			return normalized, nil
		}
	}
	return "", errors.New("configuración OIKOS no encontrada")
}

func urlEscape(s string) string {
	return strings.ReplaceAll(s, " ", "%20")
}

func obtenerProyectosPorFacultad(ctx *webctx.Context) ([]nodoOikos, error) {
	oikos, err := getOikosBaseURL()
	fmt.Println("OIKOSBASEURL", oikos)
	if err != nil {
		return nil, fmt.Errorf("configuración OikosService: %w", err)
	}
	baseURL := fmt.Sprintf("http://%s", oikos)
	url := fmt.Sprintf("%s/proyecto_curricular/get_all_proyectos_by_facultades?limit=0", strings.TrimRight(baseURL, "/"))

	var ans respOikosProys
	if err := helpers.GetJSON(ctx, url, &ans, nil); err != nil {
		return nil, err
	}
	if strings.ToLower(ans.Type) != "success" {
		return nil, nil
	}
	return ans.Body, nil
}

func flattenProyectosPorFacultad(body []nodoOikos) []OpcionDTO {
	out := make([]OpcionDTO, 0, 256)
	seen := map[int]bool{}
	for _, fac := range body {
		hijos := extraerHijosPrimerNivel(fac.Opciones, fac.Id)
		for _, p := range hijos {
			if !seen[p.Id] {
				out = append(out, p)
				seen[p.Id] = true
			}
		}
	}
	return out
}

func buscarNombreProyecto(body []nodoOikos, proyectoID int) string {
	for _, fac := range body {
		hijos := extraerHijosPrimerNivel(fac.Opciones, fac.Id)
		//fmt.Println("Hijos Primer Nivel Del Objeto", hijos)
		for _, p := range hijos {
			if p.Id == proyectoID {
				return p.Nombre
			}
		}
	}
	return ""
}

func normalizeOikosHost(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	return strings.TrimRight(trimmed, "/")
}

func listarProyectosCurricularesAll(q string) ([]OpcionDTO, error) {
	const (
		pageSize = 200
		maxItems = 2000
	)
	all := make([]OpcionDTO, 0)
	page := 1

	for {
		offset := (page - 1) * pageSize
		filters := map[string]string{
			"limit":  strconv.Itoa(pageSize),
			"offset": strconv.Itoa(offset),
		}
		if strings.TrimSpace(q) != "" {
			filters["q"] = strings.TrimSpace(q)
		}

		batch, err := rootservices.ListProyectosCurricularesWithFilters(filters)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		for _, p := range batch {
			all = append(all, OpcionDTO{Id: p.Id, Nombre: strings.TrimSpace(p.Nombre)})
			if len(all) >= maxItems {
				sortByNombre(all)
				return all, nil
			}
		}

		page++
	}

	sortByNombre(all)
	return all, nil
}
