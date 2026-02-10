package dto

// EstudiantePerfilUpsert encapsula la carga útil para crear/actualizar el perfil.
type EstudiantePerfilUpsert struct {
	ProyectoCurricularID     *int    `json:"proyecto_curricular_id,omitempty"`
	Resumen                  *string `json:"resumen,omitempty"`
	Habilidades              *string `json:"habilidades,omitempty"`
	CVDocumentoID            *string `json:"cv_documento_id,omitempty"`
	Visible                  *bool   `json:"visible,omitempty"`
	TratamientoDatosAceptado *bool   `json:"tratamiento_datos_aceptado,omitempty"`
}

// EstudiantePerfilUpsertReq agrega la llave del tercero al payload genérico.
type EstudiantePerfilUpsertReq struct {
	TerceroID *int `json:"tercero_id"`
	EstudiantePerfilUpsert
}

// EstudiantePerfilVisibilidadReq representa el cuerpo mínimo para actualizar visibilidad.
type EstudiantePerfilVisibilidadReq struct {
	TerceroID *int  `json:"tercero_id"`
	Visible   *bool `json:"visible"`
}

// EstudiantePerfilCVReq representa el cuerpo mínimo para actualizar el CV.
type EstudiantePerfilCVReq struct {
	TerceroID     *int    `json:"tercero_id"`
	CVDocumentoID *string `json:"cv_documento_id"`
}

// EstudiantePerfilDocumentoReq define el payload para consultar un perfil por documento.
type EstudiantePerfilDocumentoReq struct {
	NumeroDocumento string `json:"numero_documento"`
}

// EstudiantePerfilDocumentoResp describe la respuesta al consultar un perfil por documento.
type EstudiantePerfilDocumentoResp struct {
	Relacionado bool                   `json:"relacionado"`
	Mensaje     string                 `json:"mensaje"`
	TerceroID   *int                   `json:"tercero_id,omitempty"`
	PerfilID    *int                   `json:"perfil_id,omitempty"`
	Perfil      map[string]interface{} `json:"perfil,omitempty"`
}
