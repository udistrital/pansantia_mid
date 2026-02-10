package helpers

import (
	"net/http"

	internaldto "github.com/udistrital/pasantia_mid/internal/dto"
	"github.com/udistrital/pasantia_mid/models/requestresponse"
)

// Ok construye una respuesta estándar exitosa.
func Ok(data interface{}) internaldto.APIResponseDTO {
	return requestresponse.NewSuccess(http.StatusOK, "OK", data)
}

// Fail construye una respuesta estándar de error.
func Fail(status int, message string) internaldto.APIResponseDTO {
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	return requestresponse.NewError(status, message, nil)
}
