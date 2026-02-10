package dto

import "time"

// TutorExternoRegistroReq describe el payload para registrar tutor externo y empresa asociada.
type TutorExternoRegistroReq struct {
	NombreCompleto        string     `json:"NombreCompleto"`
	PrimerNombre          string     `json:"PrimerNombre"`
	SegundoNombre         string     `json:"SegundoNombre,omitempty"`
	PrimerApellido        string     `json:"PrimerApellido"`
	SegundoApellido       string     `json:"SegundoApellido,omitempty"`
	FechaNacimiento       *time.Time `json:"FechaNacimiento,omitempty"`
	Activo                bool       `json:"Activo"`
	TipoContribuyenteID   int        `json:"TipoContribuyenteId"`
	UsuarioWSO2           string     `json:"UsuarioWSO2"`
	TutorTipoDocumentoID  int        `json:"Tutor_TipoDocumentoId"`
	TutorNumero           string     `json:"Tutor_Numero"`
	EmpresaNombreCompleto string     `json:"Empresa_NombreCompleto"`
	EmpresaTipoDocumento  int        `json:"Empresa_TipoDocumentoId"`
	EmpresaNIT            string     `json:"Empresa_NIT"`
	ForzarCrearEmpresa    bool       `json:"ForzarCrearEmpresa,omitempty"`
}

// TutorExternoRegistroResp consolida ids generados en el registro.
type TutorExternoRegistroResp struct {
	EmpresaID      int `json:"empresa_id"`
	TutorExternoID int `json:"tutor_externo_id"`
	VinculacionID  int `json:"vinculacion_id"`
}
