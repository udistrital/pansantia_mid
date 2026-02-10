package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	rootservices "github.com/udistrital/pasantia_mid/services"
)

type pcRow struct {
	ProyectoCurricularId int `json:"proyecto_curricular_id"`
}

// getPCIDsByOferta consulta Castor CRUD para traer los PCs asociados a la oferta.
func getPCIDsByOferta(ofertaID int) ([]int, error) {
	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, "oferta_pasantia", strconv.Itoa(ofertaID), "carreras")

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d en %s", resp.StatusCode, endpoint)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if ids, ok := parsePCIDsFromWrapper(body); ok {
		return ids, nil
	}
	if ids, ok := parsePCIDsFromArray(body); ok {
		return ids, nil
	}
	return nil, fmt.Errorf("respuesta inválida al consultar PCs de la oferta %d", ofertaID)
}

func parsePCIDsFromWrapper(body []byte) ([]int, bool) {
	var wrapper struct {
		Data json.RawMessage `json:"Data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil || len(wrapper.Data) == 0 {
		return nil, false
	}
	var rows []pcRow
	if err := json.Unmarshal(wrapper.Data, &rows); err != nil {
		return nil, false
	}
	return toPCIDs(rows), true
}

func parsePCIDsFromArray(body []byte) ([]int, bool) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	var rows []pcRow
	if err := decoder.Decode(&rows); err != nil {
		return nil, false
	}
	return toPCIDs(rows), true
}

func toPCIDs(rows []pcRow) []int {
	if len(rows) == 0 {
		return []int{}
	}
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ProyectoCurricularId)
	}
	return ids
}
