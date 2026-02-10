package dto

import "time"

// OfertaCreateResp representa la respuesta consolidada al crear una oferta con proyectos curriculares.
type OfertaCreateResp struct {
	ID                    int        `json:"id"`
	FechaPublicacion      *time.Time `json:"fecha_publicacion,omitempty"`
	Titulo                string     `json:"titulo"`
	Descripcion           string     `json:"descripcion"`
	EmpresaTerceroID      int        `json:"empresa_tercero_id"`
	TutorExternoID        int        `json:"tutor_externo_id"`
	Modalidad             string     `json:"modalidad"`
	Estado                string     `json:"estado"`
	ProyectosCurriculares []int      `json:"proyectos_curriculares"`
}
