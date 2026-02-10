package helpers

import (
	"net/http"
	"os"
	"strings"
	"sync"

	roothelpers "github.com/udistrital/pasantia_mid/helpers"
	rootservices "github.com/udistrital/pasantia_mid/services"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/context"
)

type documentosClient struct{}

// Documentos expone helpers relacionados con el servicio de documentos.
var Documentos = documentosClient{}

var (
	validateDocsOnce sync.Once
	validateDocs     bool

	documentosBaseOnce sync.Once
	documentosBase     string
)

// Exists valida si un documento existe en Documentos CRUD.
func (documentosClient) Exists(ctx *context.Context, id string) (bool, error) {
	if !shouldValidateDocs() {
		return true, nil
	}
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return false, roothelpers.NewAppError(http.StatusBadRequest, "cv_documento_id inválido", nil)
	}

	base := documentosBaseURL()
	if base == "" {
		return true, nil
	}

	headers := copyRequestHeaders(ctx)
	if _, ok := headers["Authorization"]; !ok {
		headers = rootservices.AddOASAuth(headers)
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(base, "documento", trimmed)

	var payload map[string]interface{}
	if err := roothelpers.DoJSONWithHeaders("GET", endpoint, headers, nil, &payload, cfg.RequestTimeout, true); err != nil {
		if roothelpers.IsHTTPError(err, http.StatusNotFound) {
			return false, nil
		}
		return false, roothelpers.AsAppError(err, "error consultando documento")
	}
	return len(payload) > 0, nil
}

func shouldValidateDocs() bool {
	validateDocsOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv("VALIDAR_DOCS")); v != "" {
			validateDocs = strings.EqualFold(v, "true") || v == "1"
			return
		}
		validateDocs = beego.AppConfig.DefaultBool("validar_docs", false)
	})
	return validateDocs
}

func documentosBaseURL() string {
	documentosBaseOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv("DOCUMENTOS_CRUD_BASE_URL")); v != "" {
			documentosBase = v
			return
		}
		if v, err := beego.AppConfig.String("documentos_crud_base_url"); err == nil {
			documentosBase = strings.TrimSpace(v)
		}
	})
	return documentosBase
}
