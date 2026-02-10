package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/models"
)

const defaultOikosPageSize = 100

// ListProyectosCurriculares consulta el API de OIKOS aplicando filtro por nombre si se provee.
func ListProyectosCurriculares(q string) ([]models.ProyectoCurricular, error) {
	filters := map[string]string{}
	if trimmed := strings.TrimSpace(q); trimmed != "" {
		filters["q"] = trimmed
	}
	return ListProyectosCurricularesWithFilters(filters)
}

// ListProyectosCurricularesWithFilters permite aplicar filtros arbitrarios contra OIKOS soportando paginación.
func ListProyectosCurricularesWithFilters(filters map[string]string) ([]models.ProyectoCurricular, error) {
	cfg := GetConfig()
	endpoint := buildOikosURL(cfg, "dependencia")

	queryParts := []string{
		"DependenciaTipoDependencia__TipoDependenciaId__Id:2",
		"Activo:true",
	}

	values := url.Values{}
	pageSize := defaultOikosPageSize
	hasExplicitLimit := false
	offset := 0

	for key, value := range filters {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		switch strings.ToLower(key) {
		case "q", "nombre":
			queryParts = append(queryParts, fmt.Sprintf("Nombre:%s", trimmed))
		case "query":
			queryParts = append(queryParts, trimmed)
		case "limit":
			hasExplicitLimit = true
			values.Set("limit", trimmed)
			if parsed, err := strconv.Atoi(trimmed); err == nil && parsed > 0 {
				pageSize = parsed
			}
		case "offset":
			if parsed, err := strconv.Atoi(trimmed); err == nil && parsed >= 0 {
				offset = parsed
			}
		default:
			values.Set(key, trimmed)
		}
	}

	if _, present := values["limit"]; !present {
		values.Set("limit", strconv.Itoa(pageSize))
	}
	if _, present := values["fields"]; !present {
		values.Set("fields", "Id,Nombre")
	}

	headers := AddOASAuth(nil)
	results := make([]models.ProyectoCurricular, 0, pageSize)
	seen := make(map[int]struct{})

	for {
		values.Set("offset", strconv.Itoa(offset))
		values.Set("query", strings.Join(queryParts, ","))

		urlWithQuery := endpoint
		if encoded := values.Encode(); encoded != "" {
			urlWithQuery = endpoint + "?" + encoded
		}

		var page []struct {
			Id     int    `json:"Id"`
			Nombre string `json:"Nombre"`
		}
		if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &page, cfg.RequestTimeout, true); err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		for _, item := range page {
			if _, exists := seen[item.Id]; exists {
				continue
			}
			seen[item.Id] = struct{}{}
			results = append(results, models.ProyectoCurricular{
				Id:     item.Id,
				Nombre: strings.TrimSpace(item.Nombre),
			})
		}

		if hasExplicitLimit {
			break
		}
		if len(page) < pageSize {
			break
		}
		offset += len(page)
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Nombre) < strings.ToLower(results[j].Nombre)
	})

	return results, nil
}

func buildOikosURL(cfg Config, segments ...string) string {
	base := strings.TrimSpace(cfg.OikosBaseURL)
	base = strings.TrimSuffix(base, "/")
	version := strings.Trim(strings.TrimSpace(cfg.OikosVersion), "/")
	if version != "" {
		suffix := "/" + strings.ToLower(version)
		if !strings.HasSuffix(strings.ToLower(base), suffix) {
			base = BuildURL(base, version)
		}
	}
	return BuildURL(base, segments...)
}

// ListProyectosCurricularesFromHierarchy reconstruye los proyectos curriculares consultando OIKOS v1.
func ListProyectosCurricularesFromHierarchy(filters map[string]string) ([]models.ProyectoCurricular, error) {
	cfg := GetConfig()
	if cfg.OikosV1BaseURL == "" {
		return nil, helpers.NewAppError(500, "OIKOS_V1_BASE_URL no configurado", nil)
	}

	tipoID, err := fetchTipoProyectoCurricularID(cfg.OikosV1BaseURL, cfg.RequestTimeout)
	if err != nil {
		return nil, err
	}

	dependenciaIDs, err := fetchDependenciasByTipo(cfg.OikosV1BaseURL, cfg.RequestTimeout, tipoID)
	if err != nil {
		return nil, err
	}
	if len(dependenciaIDs) == 0 {
		return []models.ProyectoCurricular{}, nil
	}

	proyectos, err := fetchDependencias(cfg.OikosV1BaseURL, cfg.RequestTimeout, dependenciaIDs)
	if err != nil {
		return nil, err
	}

	if q := strings.TrimSpace(filters["q"]); q != "" {
		lower := strings.ToLower(q)
		filtered := make([]models.ProyectoCurricular, 0, len(proyectos))
		for _, p := range proyectos {
			if strings.Contains(strings.ToLower(p.Nombre), lower) {
				filtered = append(filtered, p)
			}
		}
		proyectos = filtered
	}

	sort.Slice(proyectos, func(i, j int) bool {
		left := strings.ToLower(proyectos[i].Nombre)
		right := strings.ToLower(proyectos[j].Nombre)
		if left == right {
			return proyectos[i].Id < proyectos[j].Id
		}
		return left < right
	})

	return proyectos, nil
}

func fetchTipoProyectoCurricularID(base string, timeout time.Duration) (int, error) {
	endpoint := BuildURL(base, "tipo_dependencia")
	params := url.Values{}
	params.Set("limit", "1")
	params.Set("query", "Nombre:PROYECTO CURRICULAR,Activo:true")

	urlWithQuery := endpoint + "?" + params.Encode()

	var tipos []map[string]interface{}
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, AddOASAuth(nil), nil, &tipos, timeout, true); err != nil {
		return 0, err
	}
	if len(tipos) == 0 {
		// fallback conocido
		return 1, nil
	}
	if id, ok := normalizeToInt(tipos[0]["Id"]); ok {
		return id, nil
	}
	return 1, nil
}

func fetchDependenciasByTipo(base string, timeout time.Duration, tipoID int) ([]int, error) {
	endpoint := BuildURL(base, "dependencia_tipo_dependencia")
	params := url.Values{}
	params.Set("limit", "0")
	params.Set("query", fmt.Sprintf("TipoDependenciaId.Id:%d,Activo:true", tipoID))

	urlWithQuery := endpoint + "?" + params.Encode()

	var relaciones []map[string]interface{}
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, AddOASAuth(nil), nil, &relaciones, timeout, true); err != nil {
		return nil, err
	}

	ids := make([]int, 0, len(relaciones))
	for _, rel := range relaciones {
		if depID, ok := extractNestedInt(rel, "DependenciaId", "Id"); ok {
			ids = append(ids, depID)
			continue
		}
		if depID, ok := normalizeToInt(rel["DependenciaId"]); ok {
			ids = append(ids, depID)
		}
	}
	return ids, nil
}

func fetchDependencias(base string, timeout time.Duration, ids []int) ([]models.ProyectoCurricular, error) {
	proyectos := make([]models.ProyectoCurricular, 0, len(ids))
	seen := make(map[int]struct{})
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}

		endpoint := BuildURL(base, "dependencia", strconv.Itoa(id))
		var payload map[string]interface{}
		if err := helpers.DoJSONWithHeaders("GET", endpoint, AddOASAuth(nil), nil, &payload, timeout, true); err != nil {
			if appErr, ok := err.(*helpers.AppError); ok && appErr.Status == 404 {
				continue
			}
			return nil, err
		}
		nombre := extractNombre(payload)
		proyectos = append(proyectos, models.ProyectoCurricular{Id: id, Nombre: nombre})
	}
	return proyectos, nil
}

func extractNestedInt(container map[string]interface{}, keys ...string) (int, bool) {
	current := interface{}(container)
	for _, key := range keys {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[key]
		} else {
			return 0, false
		}
	}
	return normalizeToInt(current)
}

func normalizeToInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n), true
		}
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		if n, err := strconv.Atoi(s); err == nil {
			return n, true
		}
	}
	return 0, false
}

func extractNombre(data map[string]interface{}) string {
	fmt.Println("Busqueda del Nombre de Carrera", data)
	if nombre, ok := data["Nombre"].(string); ok && nombre != "" {
		return nombre
	}
	if nombre, ok := data["NombreDependencia"].(string); ok && nombre != "" {
		return nombre
	}
	if nombre, ok := data["NombreDependenciaDependencia"].(string); ok && nombre != "" {
		return nombre
	}
	if id, ok := normalizeToInt(data["Id"]); ok {
		return fmt.Sprintf("Dependencia %d", id)
	}
	return ""
}

// GetProyectoCurricular trae un recurso puntual de OIKOS por Id usando el recurso dependencia/{id}.
func GetProyectoCurricular(id int) (*models.ProyectoCurricular, error) {
	cfg := GetConfig()
	endpoint := buildOikosURL(cfg, "dependencia", fmt.Sprintf("%d", id))
	urlWithQuery := endpoint + "?fields=Id,Nombre,NombreDependencia"
	//fmt.Println("URL OIKOS", urlWithQuery)
	var raw map[string]any
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, AddOASAuth(nil), nil, &raw, cfg.RequestTimeout, false); err != nil {
		//fmt.Println("ERROR CONSULTA OIKOS", err)
		return nil, err
	}

	// id tolerante
	pid := 0
	if v, ok := raw["Id"]; ok {
		if n, ok := normalizeToInt(v); ok {
			pid = n
		}
	} else if v, ok := raw["id"]; ok {
		if n, ok := normalizeToInt(v); ok {
			pid = n
		}
	}

	nombre := strings.TrimSpace(extractNombre(raw))
	return &models.ProyectoCurricular{Id: pid, Nombre: nombre}, nil
}
