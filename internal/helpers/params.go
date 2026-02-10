package helpers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/beego/beego/v2/server/web/context"
)

// ParamInt extrae un parámetro de ruta como entero.
func ParamInt(ctx *context.Context, name string) (int, error) {
	if ctx == nil {
		return 0, fmt.Errorf("contexto nil")
	}
	raw := strings.TrimSpace(ctx.Input.Param(name))
	if raw == "" {
		return 0, fmt.Errorf("parametro %s vacío", name)
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parametro %s inválido", name)
	}
	return val, nil
}
