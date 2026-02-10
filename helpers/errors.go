package helpers

import "fmt"

// AppError representa un error controlado con código HTTP y mensaje funcional.
type AppError struct {
	Status  int
	Message string
	Err     error
}

// Error implementa la interfaz error.
func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap permite extraer el error original cuando exista.
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewAppError construye un AppError con mensaje y status.
func NewAppError(status int, message string, err error) *AppError {
	return &AppError{Status: status, Message: message, Err: err}
}

// AsAppError convierte cualquier error en AppError con status 500 por defecto.
func AsAppError(err error, defaultMessage string) *AppError {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	msg := defaultMessage
	if msg == "" {
		msg = "error inesperado"
	}
	return &AppError{Status: 500, Message: msg, Err: err}
}
