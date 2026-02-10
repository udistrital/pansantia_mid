package dto

import (
	"github.com/udistrital/pasantia_mid/models/requestresponse"
)

// APIResponseDTO reutiliza el DTO estándar expuesto por requestresponse.
// Alias para mantener compatibilidad con consumidores existentes.
type APIResponseDTO = requestresponse.APIResponseDTO

// PageDTO representa una colección paginada.
type PageDTO[T any] struct {
	Items []T   `json:"items"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
	Total int64 `json:"total"`
}

// EstudiantePerfilCard describe la información básica mostrada en catálogos Explorador.
type EstudiantePerfilCard struct {
	PerfilID                 int64                  `json:"perfil_id"`
	TerceroID                int64                  `json:"tercero_id"`
	ProyectoCurricularID     int64                  `json:"proyecto_curricular_id"`
	ProyectoCurricularNombre string                 `json:"proyecto_curricular_nombre,omitempty"`
	ProyectoCurricular       map[string]interface{} `json:"proyecto_curricular,omitempty"`
	Resumen                  string                 `json:"resumen"`
	Habilidades              []string               `json:"habilidades,omitempty"`
	CVDocumentoID            *string                `json:"cv_documento_id,omitempty"`
	Visible                  bool                   `json:"visible"`
	Guardado                 bool                   `json:"guardado"`
	TratamientoDatosAceptado bool                   `json:"tratamiento_datos_aceptado"`
}

// PostulacionAccion encapsula la acción tomada sobre una postulación.
type PostulacionAccion struct {
	Accion     string  `json:"accion"`
	Comentario *string `json:"comentario,omitempty"`
}

// InvitacionCreate describe la solicitud para crear/invitar a un estudiante.
type InvitacionCreate struct {
	OfertaID         *int64 `json:"oferta_id,omitempty"`
	OfertaPasantiaID *int64 `json:"oferta_pasantia_id,omitempty"`
	Mensaje          string `json:"mensaje"`
}

type OptionDTO struct {
	ID     int    `json:"id"`
	Nombre string `json:"nombre"`
}
