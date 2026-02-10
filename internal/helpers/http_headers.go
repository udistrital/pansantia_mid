package helpers

import (
	"strings"

	"github.com/beego/beego/v2/server/web/context"
)

func copyRequestHeaders(ctx *context.Context) map[string]string {
	headers := make(map[string]string)
	if ctx == nil {
		return headers
	}
	if idem := strings.TrimSpace(ctx.Input.Header("Idempotency-Key")); idem != "" {
		headers["Idempotency-Key"] = idem
	}
	if auth := strings.TrimSpace(ctx.Input.Header("Authorization")); auth != "" {
		headers["Authorization"] = auth
	}
	if corr := strings.TrimSpace(ctx.Input.Header("X-Request-Id")); corr != "" {
		headers["X-Request-Id"] = corr
	}
	if corr := strings.TrimSpace(ctx.Input.Header("X-Correlation-Id")); corr != "" {
		headers["X-Correlation-Id"] = corr
	}
	return headers
}
