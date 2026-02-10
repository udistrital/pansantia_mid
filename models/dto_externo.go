package models

type EmpresaInDTO struct {
	RazonSocial         string `json:"razon_social"`          // Tercero.NombreCompleto (empresa)
	NITSinDV            string `json:"nit_sin_dv"`            // DatosIdentificacion.Numero (sin DV)
	TipoContribuyenteId int    `json:"tipo_contribuyente_id"` // id válido
	TipoDocumentoId     int    `json:"tipo_documento_id"`     // id del tipo “NIT”
}

type TutorExternoInDTO struct {
	PrimerNombre        string `json:"primer_nombre"`
	SegundoNombre       string `json:"segundo_nombre"`
	PrimerApellido      string `json:"primer_apellido"`
	SegundoApellido     string `json:"segundo_apellido"`
	FechaNacimiento     string `json:"fecha_nacimiento"` // ISO; DB es timestamp
	UsuarioWSO2         string `json:"usuario_wso2"`     // email
	Activo              bool   `json:"activo"`
	TipoContribuyenteId int    `json:"tipo_contribuyente_id"` // persona natural
	TipoDocumentoId     int    `json:"tipo_documento_id"`     // CC/CE...
	NumeroDocumento     string `json:"numero_documento"`
}

type RegistrarTutorExternoInDTO struct {
	Empresa      EmpresaInDTO      `json:"empresa"`
	TutorExterno TutorExternoInDTO `json:"tutor_externo"`
}

type RegistrarTutorExternoOutDTO struct {
	EmpresaId      int `json:"empresa_id"`
	TutorExternoId int `json:"tutor_externo_id"`
	VinculacionId  int `json:"vinculacion_id"`
}
