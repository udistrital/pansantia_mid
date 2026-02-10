package services

import "github.com/udistrital/pasantia_mid/models"

// GetEstadosCatalogo expone los estados relevantes consumiendo el servicio de parámetros.
func GetEstadosCatalogo() (models.EstadosCatalogo, error) {
	return MapEstados()
}

// GetProyectosCurriculares delega en el cliente de OIKOS con filtros arbitrarios.
func GetProyectosCurriculares(filters map[string]string) ([]models.ProyectoCurricular, error) {
	proyectos, err := ListProyectosCurricularesFromHierarchy(filters)
	if err == nil && len(proyectos) > 0 {
		return proyectos, nil
	}
	if err == nil {
		// si no se obtuvieron resultados, intentar con el endpoint v2 para mantener compatibilidad
		return ListProyectosCurricularesWithFilters(filters)
	}
	// fallback en caso de error consultando la jerarquía
	secundarios, fallbackErr := ListProyectosCurricularesWithFilters(filters)
	if fallbackErr != nil {
		return nil, err
	}
	return secundarios, nil
}
