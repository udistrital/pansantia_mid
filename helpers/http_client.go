// helpers/http_client.go
package helpers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// ---------- Utilidades URL/HEAD/GET-probe  ----------

func IsHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func DoHEAD(url string, headers map[string]string, timeout time.Duration) (int, http.Header, error) {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, resp.Header, nil
}

func DoGETProbe(url string, headers map[string]string, timeout time.Duration) (int, http.Header, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", "bytes=0-0") // sólo primer byte
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer io.Copy(io.Discard, resp.Body)
	defer resp.Body.Close()
	return resp.StatusCode, resp.Header, nil
}

// ---------- Cliente JSON (wrapped y no wrapped) + RETRIES ----------

type CrudWrapper struct {
	Success bool            `json:"Success"`
	Status  json.RawMessage `json:"Status,omitempty"`
	Message string          `json:"Message"`
	Data    json.RawMessage `json:"Data"`
}

// HTTPError envuelve códigos de estado no exitosos para permitir un manejo granular.
type HTTPError struct {
	Status int
	Body   string
}

// Error imprime el estado y cuerpo asociado.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
}

// IsHTTPError permite consultar si el error corresponde a un status específico.
func IsHTTPError(err error, status int) bool {
	if err == nil {
		return false
	}
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Status == status
	}
	return false
}

// Config global de reintentos (retro-compatible)
var (
	defaultRetryCount  = 0
	defaultBackoffBase = 300 * time.Millisecond
	maxBackoff         = 3 * time.Second
)

func SetDefaultRetryCount(n int) {
	if n < 0 {
		n = 0
	}
	defaultRetryCount = n
}

func SetRetryBackoff(baseMs int) {
	if baseMs <= 0 {
		baseMs = 300
	}
	defaultBackoffBase = time.Duration(baseMs) * time.Millisecond
}

// Retro-compatible: asume wrapped=true y sin headers
func DoJSON(method, url string, in any, out any, timeout time.Duration) error {
	return DoJSONWithHeaders(method, url, nil, in, out, timeout, true)
}

// Con headers y control de envoltura; aplica reintentos
func DoJSONWithHeaders(method, url string, headers map[string]string, in any, out any, timeout time.Duration, wrapped bool) error {
	// Serializa body una vez
	var body []byte
	var err error
	if in != nil {
		body, err = json.Marshal(in)
		if err != nil {
			return err
		}
	}

	doOnce := func() error {
		var reader io.Reader
		if body != nil {
			reader = bytes.NewBuffer(body)
		}
		req, err := http.NewRequest(method, url, reader)
		if err != nil {
			return err
		}
		if in != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			b, _ := io.ReadAll(resp.Body)
			return &HTTPError{
				Status: resp.StatusCode,
				Body:   strings.TrimSpace(string(b)),
			}
		}

		if out == nil {
			io.Copy(io.Discard, resp.Body)
			return nil
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if len(bodyBytes) == 0 {
			return nil
		}

		if wrapped {
			var w CrudWrapper
			if err := json.Unmarshal(bodyBytes, &w); err != nil {
				var ute *json.UnmarshalTypeError
				if errors.As(err, &ute) && (ute.Type == reflect.TypeOf(CrudWrapper{}) || ute.Type == reflect.TypeOf(&CrudWrapper{})) {
					return json.Unmarshal(bodyBytes, out)
				}
				return err
			}
			if !w.Success {
				if w.Message == "" {
					w.Message = "operación fallida (Success=false)"
				}
				return errors.New(w.Message)
			}
			if len(w.Data) == 0 {
				return nil
			}
			return json.Unmarshal(w.Data, out)
		}

		return json.Unmarshal(bodyBytes, out)
	}

	var attempt int
	for {
		err = doOnce()
		if err == nil {
			return nil
		}
		if attempt >= defaultRetryCount || !isRetryableErr(err) {
			return err
		}
		time.Sleep(backoffFor(attempt))
		attempt++
	}
}

func isRetryableErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	l := strings.ToLower(err.Error())
	if strings.HasPrefix(l, "http 500") || strings.HasPrefix(l, "http 502") ||
		strings.HasPrefix(l, "http 503") || strings.HasPrefix(l, "http 504") {
		return true
	}
	return strings.Contains(l, "timeout") ||
		strings.Contains(l, "connection reset") ||
		strings.Contains(l, "temporary") ||
		strings.Contains(l, "server closed idle connection")
}

func backoffFor(attempt int) time.Duration {
	d := defaultBackoffBase << attempt
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}
