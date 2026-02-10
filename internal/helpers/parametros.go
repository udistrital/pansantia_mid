package helpers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	rootservices "github.com/udistrital/pasantia_mid/services"
)

type ParametroDTO struct {
	Id                int    `json:"Id"`
	Nombre            string `json:"Nombre"`
	CodigoAbreviacion string `json:"CodigoAbreviacion"`
	Descripcion       string `json:"Descripcion"`
}

type listWrapper struct {
	Success bool        `json:"Success"`
	Status  string      `json:"Status"`
	Message interface{} `json:"Message"`
	Data    interface{} `json:"Data"`
}

// GetParametroByCodeNoCache consulta parametros_crud en:
//
//	{parametros_base_url}/parametro?query=CodigoAbreviacion__iexact:CODE&limit=1&fields=...
//
// y retorna un único registro. Sin caché.
func GetParametroByCodeNoCache(code string) (ParametroDTO, error) {
	cfg := rootservices.GetConfig()

	// base = http://pruebasapi.intranetoas.udistrital.edu.co:8510/v1
	base := strings.TrimRight(cfg.ParametrosBaseURL, "/")

	// construir URL segura: base + /parametro
	u, err := url.Parse(base)
	if err != nil {
		return ParametroDTO{}, err
	}
	u.Path = path.Join(u.Path, "/parametro")

	q := url.Values{}
	q.Set("limit", "1")
	q.Set("query", fmt.Sprintf("CodigoAbreviacion__iexact:%s", strings.TrimSpace(code)))
	// Campos en casing del modelo Beego (y solo los necesarios)
	q.Set("fields", "Id,Nombre,CodigoAbreviacion,Descripcion")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return ParametroDTO{}, err
	}
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ParametroDTO{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ParametroDTO{}, fmt.Errorf("parametros HTTP %d en %s", resp.StatusCode, u.String())
	}

	var w listWrapper
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return ParametroDTO{}, err
	}

	// Desenrollar Data (slice)
	b, _ := json.Marshal(w.Data)
	var arr []map[string]interface{}
	if err := json.Unmarshal(b, &arr); err != nil {
		return ParametroDTO{}, err
	}
	if len(arr) == 0 || (len(arr) == 1 && len(arr[0]) == 0) {
		return ParametroDTO{}, fmt.Errorf("parametro '%s' no encontrado", code)
	}

	first := arr[0]
	out := ParametroDTO{}
	if v, ok := first["Id"].(float64); ok {
		out.Id = int(v)
	}
	if v, ok := first["Nombre"].(string); ok {
		out.Nombre = v
	}
	if v, ok := first["CodigoAbreviacion"].(string); ok {
		out.CodigoAbreviacion = v
	}
	if v, ok := first["Descripcion"].(string); ok {
		out.Descripcion = v
	}
	return out, nil
}
