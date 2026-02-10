package models

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// FlexInt permite deserializar valores que pueden venir como número, string o estructura {Id: ...}.
type FlexInt int

// UnmarshalJSON soporta formatos heterogéneos en las respuestas del API de terceros.
func (fi *FlexInt) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*fi = 0
		return nil
	}
	switch trimmed[0] {
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return err
		}
		if raw, ok := obj["Id"]; ok && raw != nil {
			return fi.UnmarshalJSON(raw)
		}
		if raw, ok := obj["id"]; ok && raw != nil {
			return fi.UnmarshalJSON(raw)
		}
		*fi = 0
		return nil
	case '"':
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			*fi = 0
			return nil
		}
		v, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		*fi = FlexInt(v)
		return nil
	default:
		var v int
		if err := json.Unmarshal(trimmed, &v); err != nil {
			return err
		}
		*fi = FlexInt(v)
		return nil
	}
}

// MarshalJSON serializa el valor interno como entero.
func (fi FlexInt) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(fi))
}

// Int devuelve el valor entero nativo.
func (fi FlexInt) Int() int {
	return int(fi)
}

// Oferta representa una oferta de pasantía expuesta por el MID.
type Oferta struct {
	Id                    int64     `json:"id"`
	Titulo                string    `json:"titulo"`
	Descripcion           string    `json:"descripcion"`
	Estado                string    `json:"estado"`
	FechaPublicacion      time.Time `json:"fecha_publicacion"`
	EmpresaId             int64     `json:"empresa_id"`
	TutorExternoId        int64     `json:"tutor_externo_id"`
	ProyectoCurricularIds []int64   `json:"proyecto_curricular_ids,omitempty"`
}

// CreateOfertaDTO es el payload para crear una oferta desde el MID.
type CreateOfertaDTO struct {
	Titulo         string `json:"titulo"`
	Descripcion    string `json:"descripcion"`
	EmpresaId      int64  `json:"empresa_id"`
	TutorExternoId int64  `json:"tutor_externo_id"`
}

// UpdateOfertaDTO permite actualizar parcialmente una oferta.
type UpdateOfertaDTO struct {
	Titulo      *string `json:"titulo,omitempty"`
	Descripcion *string `json:"descripcion,omitempty"`
	// Estado corresponde al parametros.codigo_abreviacion (ej. OPC_CTR).
	Estado *string `json:"estado,omitempty"`
}

// OfertaEstadoDTO se usa para cambiar el estado de una oferta.
type OfertaEstadoDTO struct {
	// Estado corresponde al parametros.codigo_abreviacion (ej. OPCAN_CTR).
	Estado string `json:"estado"`
}

// OfertaCarreraDTO representa la relación Oferta - Proyecto Curricular.
type OfertaCarreraDTO struct {
	ProyectoCurricularId int64 `json:"proyecto_curricular_id"`
}

// Postulacion representa la postulación de un estudiante a una oferta.
type Postulacion struct {
	Id                int64  `json:"id"`
	EstudianteId      int64  `json:"estudiante_id"`
	OfertaId          int64  `json:"oferta_id"`
	EstadoPostulacion string `json:"estado_postulacion"`
	FechaPostulacion  string `json:"fecha_postulacion"`
	EnlaceDocHv       string `json:"enlace_doc_hv"`
}

// CreatePostulacionDTO es el payload mínimo necesario para crear una postulación.
type CreatePostulacionDTO struct {
	EstudianteId int64  `json:"estudiante_id"`
	OfertaId     int64  `json:"oferta_id"`
	EnlaceDocHv  string `json:"enlace_doc_hv"`
}

// RegistroTutorExternoDTO es el cuerpo esperado en /v1/externo/registro.
type RegistroTutorExternoDTO struct {
	Nombres          string           `json:"nombres"`
	Apellidos        string           `json:"apellidos"`
	Identificacion   string           `json:"identificacion"`
	Empresa          EmpresaMinimaDTO `json:"empresa"`
	Correo           string           `json:"correo,omitempty"`
	Telefono         string           `json:"telefono,omitempty"`
	TerceroPersonaId *int             `json:"tercero_persona_id,omitempty"`
}

// TutorExterno representa el recurso manejado en Castor CRUD.
type TutorExterno struct {
	Id             int64  `json:"id"`
	Nombres        string `json:"nombres"`
	Apellidos      string `json:"apellidos"`
	Identificacion string `json:"identificacion"`
	EmpresaId      int    `json:"empresa_id"`
	Correo         string `json:"correo"`
	Telefono       string `json:"telefono"`
}

// CreateTutorExternoDTO encapsula los datos requeridos para crear un tutor externo.
type CreateTutorExternoDTO struct {
	Nombres        string `json:"nombres"`
	Apellidos      string `json:"apellidos"`
	Identificacion string `json:"identificacion"`
	EmpresaId      int    `json:"empresa_id"`
	Correo         string `json:"correo,omitempty"`
	Telefono       string `json:"telefono,omitempty"`
}

// RegistroExternoResponse consolida la información de empresa y tutor externo.
type RegistroExternoResponse struct {
	Empresa *Tercero      `json:"empresa"`
	Tutor   *TutorExterno `json:"tutor_externo"`
}

// EmpresaMinimaDTO contiene los datos básicos para crear/buscar una empresa en terceros.
type EmpresaMinimaDTO struct {
	NIT    string `json:"nit"`
	Nombre string `json:"nombre"`
}
