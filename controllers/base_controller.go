package controllers

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/udistrital/pasantia_mid/helpers"
	"github.com/udistrital/pasantia_mid/models/requestresponse"

	beego "github.com/beego/beego/v2/server/web"
)

// BaseController centraliza la construcción de respuestas estándar.
type BaseController struct {
	beego.Controller
}

// RespondSuccess envuelve un payload en el formato estándar.
func (c *BaseController) RespondSuccess(status int, message string, data interface{}) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = requestresponse.NewSuccess(status, message, data)
	_ = c.ServeJSON()
}

// RespondError transforma cualquier error en la respuesta estándar.
func (c *BaseController) RespondError(err error) {
	appErr := helpers.AsAppError(err, "error inesperado")
	c.Ctx.Output.SetStatus(appErr.Status)
	c.Data["json"] = requestresponse.NewError(appErr.Status, appErr.Message, nil)
	_ = c.ServeJSON()
}

// ParseJSONBody deserializa el cuerpo de la petición en dest.
func (c *BaseController) ParseJSONBody(out interface{}) error {
	raw := c.Ctx.Input.RequestBody

	if len(raw) == 0 && c.Ctx.Request != nil && c.Ctx.Request.Body != nil {
		b, err := io.ReadAll(c.Ctx.Request.Body)
		if err != nil {
			return err
		}
		raw = b

		// cache + reinyectar
		c.Ctx.Input.RequestBody = b
		c.Ctx.Request.Body = io.NopCloser(bytes.NewBuffer(b))
	}

	return json.Unmarshal(raw, out)
}
