package services

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/udistrital/pasantia_mid/helpers"

	beego "github.com/beego/beego/v2/server/web"
)

// Config centraliza la configuración necesaria para los servicios externos.
type Config struct {
	AppName                string
	HTTPPort               int
	RunMode                string
	CastorCRUDBaseURL      string
	OikosBaseURL           string
	OikosVersion           string
	OikosV1BaseURL         string
	ParametrosBaseURL      string
	TercerosBaseURL        string
	OASBearerToken         string
	RequestTimeout         time.Duration
	RetryCount             int
	DependenciasAPIBaseURL string
}

var (
	cfg  Config
	once sync.Once
)

// GetConfig devuelve la configuración cargada desde variables de entorno o app.conf.
func GetConfig() Config {
	once.Do(func() {
		oikosBase := normalizeBase(getString("OIKOS_BASE_URL", "oikos_base_url", ""))
		if oikosBase == "" {
			oikosBase = normalizeBase(getString("OIKOS_SERVICE", "OikosService", ""))
		}
		oikosVersion := strings.Trim(getString("OIKOS_VERSION", "oikos_version", ""), "/")
		oikosVAlt := normalizeBase(firstNonEmpty(
			getString("OIKOS_V2_BASE_URL", "oikos_v2_base_url", ""),
			getString("OIKOS_V1_BASE_URL", "oikos_v1_base_url", ""),
		))
		if oikosBase == "" && oikosVAlt != "" {
			oikosBase = oikosVAlt
		}
		if oikosVAlt == "" && oikosBase != "" {
			oikosVAlt = deriveLegacyURL(oikosBase)
		}

		cfg = Config{
			AppName:                getString("APP_NAME", "appname", "pasantias_mid"),
			HTTPPort:               getInt("HTTP_PORT", "httpport", 8080),
			RunMode:                getString("RUN_MODE", "runmode", "dev"),
			CastorCRUDBaseURL:      normalizeBase(getString("CASTOR_CRUD_BASE_URL", "castor_crud_base_url", "")),
			OikosBaseURL:           oikosBase,
			OikosVersion:           oikosVersion,
			OikosV1BaseURL:         oikosVAlt,
			ParametrosBaseURL:      normalizeBase(getString("PARAMETROS_BASE_URL", "parametros_base_url", "")),
			TercerosBaseURL:        normalizeBase(getString("TERCEROS_BASE_URL", "terceros_base_url", "")),
			OASBearerToken:         getString("OAS_BEARER_TOKEN", "oas_bearer_token", ""),
			RequestTimeout:         time.Duration(getInt("REQUEST_TIMEOUT_MS", "request_timeout_ms", 10000)) * time.Millisecond,
			RetryCount:             getInt("RETRY_COUNT", "retry_count", 2),
			DependenciasAPIBaseURL: normalizeBase(getString("DEPENDENCIAS_API_URL", "dependencias_api_base_url", "")),
		}

		if cfg.CastorCRUDBaseURL == "" {
			panic("CASTOR_CRUD_BASE_URL no configurado")
		}
		if cfg.OikosBaseURL == "" {
			panic("OIKOS_BASE_URL no configurado")
		}
		if cfg.ParametrosBaseURL == "" {
			panic("PARAMETROS_BASE_URL no configurado")
		}
		if cfg.TercerosBaseURL == "" {
			panic("TERCEROS_BASE_URL no configurado")
		}

		helpers.SetDefaultRetryCount(cfg.RetryCount)
	})
	return cfg
}

func getString(envKey, confKey, def string) string {
	if val := strings.TrimSpace(os.Getenv(envKey)); val != "" {
		return val
	}
	if val, err := beego.AppConfig.String(confKey); err == nil && strings.TrimSpace(val) != "" {
		return val
	}
	return def
}

func getInt(envKey, confKey string, def int) int {
	if val := strings.TrimSpace(os.Getenv(envKey)); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	if val, err := beego.AppConfig.Int(confKey); err == nil {
		return val
	}
	return def
}

func normalizeBase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return trimmed
}

func deriveLegacyURL(v2Base string) string {
	if strings.Contains(v2Base, "/v2/") {
		return strings.Replace(v2Base, "/v2/", "/v1/", 1)
	}
	if strings.HasSuffix(v2Base, "/v2") {
		return strings.TrimSuffix(v2Base, "v2") + "v1"
	}
	return v2Base
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// BuildURL compone una URL asegurando que no haya dobles slashes.
func BuildURL(base string, elems ...string) string {
	trimmed := strings.TrimSuffix(base, "/")
	for _, e := range elems {
		trimmed += "/" + strings.Trim(e, "/")
	}
	return trimmed
}

// MustBuildURL es un helper para construir URLs y fallar rápido en caso de base vacía.
func MustBuildURL(base string, elems ...string) string {
	if base == "" {
		panic("base URL vacía")
	}
	return BuildURL(base, elems...)
}
