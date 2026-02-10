package services

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/udistrital/pasantia_mid/helpers"

	beego "github.com/beego/beego/v2/server/web"
)

var enlaceHashRe = regexp.MustCompile(`^[A-Za-z0-9_\-=/\.]{10,}$`)

func docsBase() string {
	if v := os.Getenv("DOCUMENTOS_BASE_URL"); v != "" {
		return v
	}
	v, _ := beego.AppConfig.String("DocumentosBaseUrl")
	return v
}

func docsVersion() string {
	if v := os.Getenv("DOCUMENTOS_VERSION"); v != "" {
		return v
	}
	v, _ := beego.AppConfig.String("DocumentosVersion")
	if v == "" {
		v = "v1"
	}
	return v
}

func docsTimeout() time.Duration {
	if ms, err := beego.AppConfig.Int64("DocsTimeoutMs"); err == nil && ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	// por defecto 8s
	return 8 * time.Second
}

func docsHeaders() map[string]string {
	h := map[string]string{}

	if t := os.Getenv("DOCUMENTOS_BEARER"); t != "" {
		h["Authorization"] = "Bearer " + t
	}
	return h
}

// ValidateEnlaceExiste valida que el enlace sea reachable.
// - URL http/https: HEAD y fallback GET-probe (acepta 200/204/206 o 3xx)
// - Hash: GET metadata al servicio de documentos; acepta 200 con exists=true o url no vacía.
// Retorna la URL "normalizada" (si desde hash resolvemos una url pública); si no, retorna el mismo hash.
func ValidateEnlaceExiste(enlace string) (string, error) {
	if helpers.IsHTTPURL(enlace) {
		if okHTTPURL(enlace) {
			return enlace, nil
		}
		return "", fmt.Errorf("no se pudo verificar EnlaceDocHv (URL)")
	}

	// Validación de hash (formato)
	if !enlaceHashRe.MatchString(enlace) {
		return "", fmt.Errorf("hash de EnlaceDocHv inválido")
	}
	if okHash(enlace) {
		// Si el servicio provee URL pública
		if url := resolveURLFromHash(enlace); url != "" {
			return url, nil
		}
		// Si no, guarda el hash
		return enlace, nil
	}
	return "", fmt.Errorf("no se pudo verificar EnlaceDocHv (hash)")
}

func okHTTPURL(url string) bool {
	// HEAD
	if st, _, err := helpers.DoHEAD(url, nil, docsTimeout()); err == nil {
		// 2xx
		if st == 200 || st == 204 || st == 206 {
			return true
		}
		// 3xx (aceptamos redirect, el cliente seguirá al descargar)
		if st >= 300 && st < 400 {
			return true
		}
	}
	// GET-probe (primer byte)
	if st, _, err := helpers.DoGETProbe(url, nil, docsTimeout()); err == nil {
		return st == 200 || st == 206
	}
	return false
}

// --------- Integra servicio de Documentos ---------

// Ajusta estas estructuras y ruta según tu servicio real.
type docMeta struct {
	Url    string `json:"url"`
	Exists bool   `json:"exists"`
	// otros campos opcionales...
}

// okHash: consulta metadata del hash y valida existencia
func okHash(hash string) bool {
	base := docsBase()
	if base == "" {
		return false
	}
	url := fmt.Sprintf("%s/%s/documentos/%s", base, docsVersion(), hash)
	var meta docMeta
	if err := helpers.DoJSONWithHeaders("GET", url, docsHeaders(), nil, &meta, docsTimeout(), false /* no wrapped */); err != nil {
		return false
	}
	// acepta exists=true o url pública no vacía
	return meta.Exists || meta.Url != ""
}

// resolveURLFromHash: si el servicio devuelve una url pública
func resolveURLFromHash(hash string) string {
	base := docsBase()
	if base == "" {
		return ""
	}
	url := fmt.Sprintf("%s/%s/documentos/%s", base, docsVersion(), hash)
	var meta docMeta
	if err := helpers.DoJSONWithHeaders("GET", url, docsHeaders(), nil, &meta, docsTimeout(), false); err == nil {
		return meta.Url
	}
	return ""
}

// Si el servicio de Documentos retorna una respuesta **envuelta**, cambiar el último parámetro a true y
// adapta `docMeta` a la estructura en `Data`.
