package requestresponse

// APIResponseDTO encapsula la respuesta estándar del MID.
type APIResponseDTO struct {
	Success bool        `json:"Success"`
	Status  int         `json:"Status"`
	Message string      `json:"Message"`
	Data    interface{} `json:"Data"`
}

// NewSuccess construye una respuesta exitosa.
func NewSuccess(status int, message string, data interface{}) APIResponseDTO {
	if message == "" {
		message = "OK"
	}
	return APIResponseDTO{
		Success: true,
		Status:  status,
		Message: message,
		Data:    data,
	}
}

// NewError construye una respuesta de error.
func NewError(status int, message string, data interface{}) APIResponseDTO {
	if message == "" {
		message = "Error"
	}
	return APIResponseDTO{
		Success: false,
		Status:  status,
		Message: message,
		Data:    data,
	}
}
