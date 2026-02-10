package models

// Parametro representa un registro del API de parámetros.
type Parametro struct {
	Id                int     `json:"Id"`
	Nombre            string  `json:"Nombre"`
	CodigoAbreviacion string  `json:"CodigoAbreviacion"`
	NumeroOrden       float64 `json:"NumeroOrden"`
	Activo            bool    `json:"Activo"`
}

// TipoParametro representa el catálogo principal de parámetros.
type TipoParametro struct {
	Id                int    `json:"Id"`
	Nombre            string `json:"Nombre"`
	CodigoAbreviacion string `json:"CodigoAbreviacion"`
}

// EstadosCatalogo agrupa los estados relevantes para el dominio de pasantías.
type EstadosCatalogo struct {
	Oferta      map[string]int `json:"oferta"`
	Postulacion map[string]int `json:"postulacion"`
	Tutoria     map[string]int `json:"tutoria"`
}

// ProyectoCurricular es la representación mínima del recurso de OIKOS.
type ProyectoCurricular struct {
	Id     int    `json:"Id"`
	Nombre string `json:"Nombre"`
}
