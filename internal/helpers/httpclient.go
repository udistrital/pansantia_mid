package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	webctx "github.com/beego/beego/v2/server/web/context"
)

var httpClient = &http.Client{
	Timeout: 20 * time.Second,
}

// GetJSON hace un GET y decodifica JSON en `out`.
// - ctx: contexto de Beego del request actual (para correlación y cancelación).
// - url: endpoint HTTP a consultar.
// - out: puntero al struct/slice donde decodificar la respuesta.
// - extra: headers adicionales (opcional). Ej: map[string]string{"X-API-Key":"..."}.
func GetJSON(ctx *webctx.Context, url string, out interface{}, extra map[string]string) error {
	// Usar el contexto del request para cancelación/timeout upstream.
	var stdctx context.Context
	if ctx != nil && ctx.Request != nil {
		stdctx = ctx.Request.Context()
	} else {
		stdctx = context.Background()
	}

	req, err := http.NewRequestWithContext(stdctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	// Headers básicos
	req.Header.Set("Accept", "application/json")

	// Propagar correlación y auth si vienen del request entrante
	if ctx != nil {
		if corr := ctx.Input.Header("X-Correlation-Id"); corr != "" {
			req.Header.Set("X-Correlation-Id", corr)
		}
		if auth := ctx.Input.Header("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}
	}

	// Headers extra opcionales
	for k, v := range extra {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s -> %d: %s", url, resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func GetText(ctx *webctx.Context, url string, extra map[string]string) (string, error) {
	var stdctx context.Context
	if ctx != nil && ctx.Request != nil {
		stdctx = ctx.Request.Context()
	} else {
		stdctx = context.Background()
	}

	req, err := http.NewRequestWithContext(stdctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Accept", "*/*")

	if ctx != nil {
		if corr := ctx.Input.Header("X-Correlation-Id"); corr != "" {
			req.Header.Set("X-Correlation-Id", corr)
		}
		if auth := ctx.Input.Header("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}
	}

	for k, v := range extra {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s -> %d: %s", url, resp.StatusCode, string(b))
	}

	return string(b), nil
}
