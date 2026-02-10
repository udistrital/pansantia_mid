package models

// Tercero representa el recurso básico proveniente del API de terceros.
type Tercero struct {
	Id              int    `json:"Id"`
	Nombre          string `json:"Nombre,omitempty"`
	NombreCompleto  string `json:"NombreCompleto,omitempty"`
	PrimerNombre    string `json:"PrimerNombre,omitempty"`
	SegundoNombre   string `json:"SegundoNombre,omitempty"`
	PrimerApellido  string `json:"PrimerApellido,omitempty"`
	SegundoApellido string `json:"SegundoApellido,omitempty"`
}

// DatosIdentificacion representa la data principal del recurso datos_identificacion.
type DatosIdentificacion struct {
	Id              int     `json:"Id"`
	Numero          string  `json:"Numero"`
	TerceroId       int     `json:"TerceroId"`
	TipoDocumentoId FlexInt `json:"TipoDocumentoId"`
	Activo          bool    `json:"Activo"`
}

// Vinculacion describe la relación entre un tercero principal y uno relacionado.
type Vinculacion struct {
	Id                   int  `json:"Id,omitempty"`
	TerceroPrincipalId   int  `json:"TerceroPrincipalId"`
	TerceroRelacionadoId int  `json:"TerceroRelacionadoId"`
	Activo               bool `json:"Activo"`
}
