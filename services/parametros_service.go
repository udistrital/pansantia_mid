package services

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/models"
)

type cacheEntry struct {
	value      interface{}
	expiration time.Time
}

var (
	tipoParametroCache sync.Map
	parametrosCache    sync.Map
	estadosCache       sync.Map
)

// GetTipoParametroId obtiene el Id de un tipo de parámetro dado su código abreviado.
func GetTipoParametroId(codigo string) (int, error) {
	key := strings.ToUpper(strings.TrimSpace(codigo))
	if key == "" {
		return 0, fmt.Errorf("codigo vacío")
	}
	if entry, ok := getFromCache(&tipoParametroCache, key); ok {
		if id, okCast := entry.(int); okCast {
			return id, nil
		}
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.ParametrosBaseURL, "tipo_parametro")
	values := url.Values{}
	values.Set("query", fmt.Sprintf("CodigoAbreviacion:%s,Activo:true", key))
	urlWithQuery := endpoint + "?" + values.Encode()

	var response []models.TipoParametro
	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &response, cfg.RequestTimeout, true); err != nil {
		return 0, err
	}
	if len(response) == 0 {
		return 0, helpers.NewAppError(404, fmt.Sprintf("tipo parametro %s no encontrado", key), nil)
	}

	tipoID := response[0].Id
	saveInCache(&tipoParametroCache, key, tipoID, 10*time.Minute)
	return tipoID, nil
}

// ListParametrosByTipo retorna los parámetros activos de un tipo.
func ListParametrosByTipo(codigoTipo string) ([]models.Parametro, error) {
	key := strings.ToUpper(strings.TrimSpace(codigoTipo))
	if cached, ok := getFromCache(&parametrosCache, key); ok {
		if data, okCast := cached.([]models.Parametro); okCast {
			return data, nil
		}
	}

	tipoID, err := GetTipoParametroId(key)
	if err != nil {
		return nil, err
	}

	cfg := GetConfig()
	endpoint := BuildURL(cfg.ParametrosBaseURL, "parametro")
	values := url.Values{}
	values.Set("query", fmt.Sprintf("TipoParametroId.Id:%d,Activo:true", tipoID))
	values.Set("limit", "0")
	urlWithQuery := endpoint + "?" + values.Encode()

	var response []models.Parametro
	headers := AddOASAuth(nil)
	if err := helpers.DoJSONWithHeaders("GET", urlWithQuery, headers, nil, &response, cfg.RequestTimeout, true); err != nil {
		return nil, err
	}

	saveInCache(&parametrosCache, key, response, 10*time.Minute)
	return response, nil
}

// MapEstados retorna el catálogo agrupado de estados relevantes.
const estadosCacheKey = "catalogo_estados"

func MapEstados() (models.EstadosCatalogo, error) {
	if cached, ok := getFromCache(&estadosCache, estadosCacheKey); ok {
		if data, okCast := cached.(models.EstadosCatalogo); okCast {
			return data, nil
		}
	}

	oferta, err := makeMapFromParametros("ESTADO_OFERTA")
	if err != nil {
		return models.EstadosCatalogo{}, err
	}
	postulacion, err := makeMapFromParametros("ESTADO_POSTULACION")
	if err != nil {
		return models.EstadosCatalogo{}, err
	}
	tutoria, err := makeMapFromParametros("ESTADO_TUTORIA_INTERNA")
	if err != nil {
		return models.EstadosCatalogo{}, err
	}

	catalogo := models.EstadosCatalogo{
		Oferta:      oferta,
		Postulacion: postulacion,
		Tutoria:     tutoria,
	}
	saveInCache(&estadosCache, estadosCacheKey, catalogo, 10*time.Minute)
	return catalogo, nil
}

func makeMapFromParametros(tipo string) (map[string]int, error) {
	registros, err := ListParametrosByTipo(tipo)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int, len(registros))
	for _, r := range registros {
		if strings.TrimSpace(r.CodigoAbreviacion) == "" {
			continue
		}
		result[strings.ToLower(r.CodigoAbreviacion)] = r.Id
	}
	return result, nil
}

func getFromCache(store *sync.Map, key string) (interface{}, bool) {
	if value, ok := store.Load(key); ok {
		if entry, okEntry := value.(cacheEntry); okEntry {
			if time.Now().Before(entry.expiration) {
				return entry.value, true
			}
			store.Delete(key)
		}
	}
	return nil, false
}

func saveInCache(store *sync.Map, key string, value interface{}, ttl time.Duration) {
	store.Store(key, cacheEntry{value: value, expiration: time.Now().Add(ttl)})
}
