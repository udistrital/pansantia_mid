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

type notificacionesClient struct{}

// Notificaciones expone el wrapper al servicio de notificaciones.
var Notificaciones = notificacionesClient{}

var (
	notificacionesBaseOnce sync.Once
	notificacionesBase     string
)

// Send dispara una notificación hacia un tercero.
func (notificacionesClient) Send(ctx *context.Context, toTerceroID int, asunto, plantilla string, data interface{}) error {
	base := notificacionesBaseURL()
	if base == "" {
		return nil
	}
	if toTerceroID <= 0 {
		return roothelpers.NewAppError(http.StatusBadRequest, "tercero destino inválido", nil)
	}

	headers := copyRequestHeaders(ctx)
	if _, ok := headers["Authorization"]; !ok {
		headers = rootservices.AddOASAuth(headers)
	}

	body := map[string]interface{}{
		"TerceroId": toTerceroID,
		"Asunto":    strings.TrimSpace(asunto),
		"Plantilla": strings.TrimSpace(plantilla),
		"Datos":     data,
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(base, "notificaciones")
	var response map[string]interface{}
	if err := roothelpers.DoJSONWithHeaders("POST", endpoint, headers, body, &response, cfg.RequestTimeout, true); err != nil {
		return roothelpers.AsAppError(err, "error enviando notificación")
	}
	return nil
}

func notificacionesBaseURL() string {
	notificacionesBaseOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv("NOTIFICACIONES_BASE_URL")); v != "" {
			notificacionesBase = v
			return
		}
		if v, err := beego.AppConfig.String("notificaciones_base_url"); err == nil {
			notificacionesBase = strings.TrimSpace(v)
		}
	})
	return notificacionesBase
}
