package helpers

// APIResponse es el formato de respuesta estándar del MID.
type APIResponse struct {
	Success bool        `json:"Success"`
	Status  int         `json:"Status"`
	Message string      `json:"Message"`
	Data    interface{} `json:"Data"`
}

// NewSuccessResponse crea una respuesta satisfactoria.
func NewSuccessResponse(status int, message string, data interface{}) APIResponse {
	if message == "" {
		message = "OK"
	}
	return APIResponse{
		Success: true,
		Status:  status,
		Message: message,
		Data:    data,
	}
}

// NewErrorResponse crea una respuesta de error.
func NewErrorResponse(status int, message string, data interface{}) APIResponse {
	if message == "" {
		message = "Error"
	}
	return APIResponse{
		Success: false,
		Status:  status,
		Message: message,
		Data:    data,
	}
}
