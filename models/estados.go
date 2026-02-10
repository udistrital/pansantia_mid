package models

// Estados de oferta y postulación basados en parametro.codigo_abreviacion.
const (
	OfertaEstadoCreada     = "OPC_CTR"
	OfertaEstadoCancelada  = "OPCAN_CTR"
	OfertaEstadoEnCurso    = "OPCUR_CTR"
	OfertaEstadoPausada    = "OPPAU_CTR"
	PostEstadoEnviada      = "PPE_CTR"
	PostEstadoDescartada   = "PSRJ_CTR"
	PostEstadoAceptada     = "PSAC_CTR"
	PostEstadoPorRevisar   = "PSPO_CTR"
	PostEstadoRevisada     = "PSRV_CTR"
	PostEstadoRechazada    = "PSRJ_CTR"
	PostEstadoSeleccionada = "PSSE_CTR"
)

// Alias conservados temporalmente para compatibilidad con código existente.
const (
	OfertaEstadoAbiertaCodigo   = OfertaEstadoCreada
	OfertaEstadoCanceladaCodigo = OfertaEstadoCancelada
	OfertaEstadoEnCursoCodigo   = OfertaEstadoEnCurso
)
