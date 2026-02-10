package middlewares

import (
	"sync"

	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/context"
)

var (
	authOnce sync.Once
)

// UseAuth registra el middleware de autenticación opcionalmente una sola vez.
func UseAuth() {
	authOnce.Do(func() {
		beego.InsertFilter("/*", beego.BeforeRouter, authFilter)
	})
}

// AuthFilter expone el filtro para escenarios donde el registro manual sea preferido.
func AuthFilter(ctx *context.Context) {
	authFilter(ctx)
}

func authFilter(ctx *context.Context) {
	// Intentar cargar los claims y dejar el error en contexto solo si no es ausencia de header.
	if _, err := internalhelpers.Claims(ctx); err != nil && err != internalhelpers.ErrNoAuthHeader {
		ctx.Input.SetData("auth_error", err)
	}
}
