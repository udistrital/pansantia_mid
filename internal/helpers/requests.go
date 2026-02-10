package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/beego/beego/v2/server/web/context"
)

// NewJSONRequest construye una petición HTTP propagando cabeceras básicas desde el contexto.
func NewJSONRequest(ctx *context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if ctx != nil {
		for key, values := range ctx.Request.Header {
			if len(values) == 0 {
				continue
			}
			// Evitar sobrescribir Content-Type cuando ya lo definió
			if strings.EqualFold(key, "Content-Type") {
				continue
			}
			req.Header[key] = append([]string(nil), values...)
		}
	}

	return req, nil
}

// DoJSON ejecuta la petición y deserializa la respuesta JSON en out (si se provee).
func DoJSON(req *http.Request, out interface{}) error {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(buf.String()))
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
