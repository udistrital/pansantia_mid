package errorhandler

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/udistrital/pasantia_mid/models/requestresponse"

	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
)

// ErrorHandlerController se registra en el router para gestionar 404 y otros fallos.
type ErrorHandlerController struct {
	beego.Controller
}

// Error404 centraliza la respuesta cuando la ruta no existe.
func (c *ErrorHandlerController) Error404() {
	method := c.Ctx.Request.Method
	path := c.Ctx.Request.URL.Path
	status := http.StatusNotFound
	message := fmt.Sprintf("nomatch|%s|%s", method, path)

	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = requestresponse.NewError(status, message, nil)
	_ = c.ServeJSON()
}

// HandlePanic captura pánicos en controladores y entrega una respuesta estándar.
func HandlePanic(ctrl *beego.Controller) {
	if r := recover(); r != nil {
		logs.Error("panic:", r)
		debug.PrintStack()

		appName := beego.AppConfig.DefaultString("appname", "pasantia_mid")
		message := fmt.Sprintf("Error service %s: An internal server error occurred.", appName)
		message += fmt.Sprintf(" Request Info: URL: %s, Method: %s", ctrl.Ctx.Request.URL, ctrl.Ctx.Request.Method)
		message += " Time: " + time.Now().UTC().Format(time.RFC3339)

		status := http.StatusInternalServerError
		ctrl.Ctx.Output.SetStatus(status)
		ctrl.Data["json"] = requestresponse.NewError(status, message, nil)
		_ = ctrl.ServeJSON()
	}
}
