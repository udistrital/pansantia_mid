package helpers

import (
	"strconv"
	"strings"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// ParsePageSize convierte los parámetros de paginación a enteros aplicando defaults y tope.
func ParsePageSize(pageStr, sizeStr string) (int, int) {
	page := defaultPage
	size := defaultPageSize

	if v, err := strconv.Atoi(strings.TrimSpace(pageStr)); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(sizeStr)); err == nil && v > 0 {
		size = v
	}
	if size > maxPageSize {
		size = maxPageSize
	}
	return page, size
}
