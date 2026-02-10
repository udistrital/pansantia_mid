package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/udistrital/pasantia_mid/internal/clients"
	internalhelpers "github.com/udistrital/pasantia_mid/internal/helpers"
	"github.com/udistrital/pasantia_mid/models"
	rootservices "github.com/udistrital/pasantia_mid/services"
)

const (
	recentPostLimit        = 5
	recommendedOffersLimit = 5
)

// GetDashboardEstudiante retorna la información consolidada del home del estudiante.
func GetDashboardEstudiante(ctx context.Context, estudianteID int) (map[string]interface{}, error) {
	crud := clients.CastorCRUD()

	resumen := map[string]interface{}{}

	// Postulaciones por estado
	postCounts, _ := crud.CountPostulacionesByEstado(ctx, map[string]string{
		"EstudianteId": fmt.Sprint(estudianteID),
	})
	resumen["postulaciones"] = postCounts
	resumen["postulaciones_por_estado"] = mapPostCountsToNamedChips(postCounts)

	// Invitaciones + visitas
	invitCounts := map[string]int{}

	visitasPerfil := map[string]interface{}{
		"total": 0,
		"items": []map[string]interface{}{},
	}

	var pcID int
	var perfilVisible interface{}
	if perfil, err := crud.GetPerfilByTerceroID(ctx, estudianteID); err == nil && perfil != nil {
		pcID = perfil.ProyectoCurricularId
		perfilVisible = perfil.Visible

		invitCounts, _ = crud.CountInvitacionesByEstado(ctx, map[string]string{
			"PerfilEstudianteId": fmt.Sprint(perfil.Id),
		})

		// 1) Intentar resumen desde CRUD
		if v, err := crud.ResumenVisitasPerfil(ctx, perfil.Id, 5 /* top */); err == nil && v != nil {
			visitasPerfil = v
		}

		// 2) Fallback: si resumen viene vacío o con tutor_id inválido (ej. 0), arma resumen desde visitas crudas
		if needsVisitasFallback(visitasPerfil) {
			if list, err := crud.ListPerfilVisitas(ctx, perfil.Id); err == nil && len(list) > 0 {
				visitasPerfil = buildResumenVisitasFromList(list, 5)
			}
		}

		// 3) Enriquecer (tutor nombre + empresa nombre) sin romper si falla
		enrichVisitasPerfil(ctx, visitasPerfil)
	}

	resumen["invitaciones"] = invitCounts
	resumen["invitaciones_por_estado"] = mapInvCountsToNamedChips(invitCounts)
	resumen["visitas_perfil"] = visitasPerfil
	resumen["perfil_visible"] = perfilVisible

	// Ofertas disponibles (KPI + chips)
	ofTotal, ofChips := buildOfertasDisponiblesResumen(ctx, estudianteID, pcID)
	resumen["ofertas_disponibles"] = ofTotal
	resumen["ofertas_disponibles_por_estado"] = ofChips

	pasanteActivo, pasantiaActiva := resolvePasantiaActiva(ctx, estudianteID)
	resumen["pasante_activo"] = pasanteActivo
	if pasanteActivo {
		resumen["pasantia_activa"] = pasantiaActiva
	}

	postulacionesRecientes := buildPostulacionesRecientes(ctx, estudianteID)
	ofertasRecomendadas := buildOfertasRecomendadas(ctx, estudianteID, pcID)

	return map[string]interface{}{
		"resumen":                     resumen,
		"mis_postulaciones_recientes": postulacionesRecientes,
		"ofertas_recomendadas":        ofertasRecomendadas,
	}, nil
}

func resolvePasantiaActiva(ctx context.Context, estudianteID int) (bool, map[string]interface{}) {
	if estudianteID <= 0 {
		return false, nil
	}

	list, err := clients.CastorCRUD().ListPostulaciones(ctx, map[string]string{
		"EstudianteId": fmt.Sprint(estudianteID),
	})
	if err != nil || len(list) == 0 {
		return false, nil
	}

	for _, post := range list {
		estadoPost := strings.ToUpper(strings.TrimSpace(post.EstadoPostulacion))
		if estadoPost != "PSAC_CTR" {
			continue
		}

		oferta, err := rootservices.GetOferta(post.OfertaId)
		if err != nil || oferta == nil {
			continue
		}

		estadoOferta := strings.ToUpper(strings.TrimSpace(oferta.Estado))
		if estadoOferta != "OPCUR_CTR" {
			continue
		}

		titulo := strings.TrimSpace(oferta.Titulo)
		if titulo == "" {
			titulo = fmt.Sprintf("Oferta #%d", post.OfertaId)
		}

		out := map[string]interface{}{
			"postulacion_id":     post.Id,
			"estado_postulacion": strings.TrimSpace(post.EstadoPostulacion),
			"oferta_id":          post.OfertaId,
			"titulo_oferta":      titulo,
			"estado_oferta":      strings.TrimSpace(oferta.Estado),
		}
		if oferta.TutorExternoId > 0 {
			out["tutor_externo_id"] = oferta.TutorExternoId
		}
		if oferta.EmpresaId > 0 {
			out["empresa_id"] = oferta.EmpresaId
		}

		out["estado_postulacion_det"] = translateEstado(post.EstadoPostulacion)
		out["estado_oferta_det"] = translateEstado(oferta.Estado)

		return true, out
	}

	return false, nil
}

// GetDashboardTutor retorna contadores básicos para el tutor.
func GetDashboardTutor(ctx context.Context, tutorID int) (map[string]interface{}, error) {
	crud := clients.CastorCRUD()

	// Ofertas del tutor por estado
	ofertas, errResumen := crud.ResumenOfertasTutor(ctx, tutorID)
	if errResumen != nil {
		return nil, errResumen
	}

	invitaciones := map[string]int{}
	if invitaciones == nil {
		invitaciones = map[string]int{}
	}

	if ofertas == nil {
		ofertas = map[string]int{}
	}

	// Postulaciones por estado + por oferta (solo de ofertas del tutor)
	postByEstado := map[string]int{}
	postByOferta := map[string]int{}
	offersList, err := crud.ListOfertasTutor(ctx, tutorID)
	if err != nil {
		return nil, err
	}
	for _, oferta := range offersList {
		if oferta.Id <= 0 {
			continue
		}
		list, err := crud.ListPostulaciones(ctx, map[string]string{
			"oferta_id": fmt.Sprint(oferta.Id),
		})
		if err != nil {
			fmt.Println("WARN dashboard tutor: error postulaciones por oferta", "oferta_id", oferta.Id, "err", err)
			continue
		}
		postByOferta[strconv.FormatInt(oferta.Id, 10)] = len(list)
		for _, p := range list {
			code := strings.ToUpper(strings.TrimSpace(p.EstadoPostulacion))
			if code == "" {
				continue
			}
			postByEstado[code]++
		}
	}
	postulaciones := map[string]interface{}{
		"por_estado": postByEstado,
		"por_oferta": postByOferta,
	}

	return map[string]interface{}{
		"ofertas":       ofertas,
		"invitaciones":  invitaciones,
		"postulaciones": postulaciones,
	}, nil
}

func buildPostulacionesRecientes(ctx context.Context, estudianteID int) []map[string]interface{} {
	crud := clients.CastorCRUD()
	list, err := crud.ListPostulaciones(ctx, map[string]string{
		"EstudianteId": fmt.Sprint(estudianteID),
	})
	if err != nil || len(list) == 0 {
		return []map[string]interface{}{}
	}

	sort.Slice(list, func(i, j int) bool {
		ti := parseTime(list[i].FechaPostulacion)
		tj := parseTime(list[j].FechaPostulacion)
		return ti.After(tj)
	})

	limit := recentPostLimit
	if len(list) < limit {
		limit = len(list)
	}

	result := make([]map[string]interface{}, 0, limit)
	ofertaCache := make(map[int64]*models.Oferta)
	for _, post := range list[:limit] {
		entry := map[string]interface{}{
			"id":                post.Id,
			"oferta_id":         post.OfertaId,
			"estado":            strings.TrimSpace(post.EstadoPostulacion),
			"estado_det":        translateEstado(post.EstadoPostulacion),
			"fecha_postulacion": strings.TrimSpace(post.FechaPostulacion),
		}
		if ofertaCache[post.OfertaId] == nil {
			if det, err := rootservices.GetOferta(post.OfertaId); err == nil && det != nil {
				ofertaCache[post.OfertaId] = det
			}
		}
		if det := ofertaCache[post.OfertaId]; det != nil {
			entry["titulo_oferta"] = strings.TrimSpace(det.Titulo)
		}
		result = append(result, entry)
	}
	return result
}

func needsVisitasFallback(visitas map[string]interface{}) bool {
	if visitas == nil {
		return true
	}
	items, _ := visitas["items"].([]map[string]interface{})
	if len(items) == 0 {
		// Si total > 0 pero items vacío => sospechoso, pero igual fallback no es obligatorio.
		// Aquí se hace para tener detalle siempre.
		if t, ok := normalizeToInt(visitas["total"]); ok && t > 0 {
			return true
		}
		return true
	}
	// si primer item trae tutor_id <= 0, el resumen está malo (caso tutor_id:0)
	first := items[0]
	tid, _ := normalizeToInt(first["tutor_id"])
	return tid <= 0
}

func buildResumenVisitasFromList(list []clients.PerfilVisita, top int) map[string]interface{} {
	if top <= 0 {
		top = 5
	}

	type agg struct {
		TutorID      int
		Total        int
		UltimaVisita time.Time
	}

	m := map[int]*agg{}
	for _, v := range list {
		tid := v.TutorId
		if tid <= 0 {
			continue
		}
		a := m[tid]
		if a == nil {
			a = &agg{TutorID: tid}
			m[tid] = a
		}
		a.Total++
		if v.FechaVisita.After(a.UltimaVisita) {
			a.UltimaVisita = v.FechaVisita
		}
	}

	items := make([]map[string]interface{}, 0, len(m))
	total := 0
	for _, a := range m {
		total += a.Total
		items = append(items, map[string]interface{}{
			"tutor_id":      a.TutorID,
			"total":         a.Total,
			"ultima_visita": a.UltimaVisita.Format(time.RFC3339),
		})
	}

	// ordenar por ultima_visita desc
	sort.SliceStable(items, func(i, j int) bool {
		return fmt.Sprint(items[i]["ultima_visita"]) > fmt.Sprint(items[j]["ultima_visita"])
	})

	if len(items) > top {
		items = items[:top]
	}

	return map[string]interface{}{
		"total": total,
		"items": items,
	}
}

func enrichVisitasPerfil(ctx context.Context, visitas map[string]interface{}) {
	if visitas == nil {
		return
	}

	itemsAny, ok := visitas["items"]
	if !ok || itemsAny == nil {
		return
	}

	items, ok := itemsAny.([]map[string]interface{})
	if !ok || len(items) == 0 {
		return
	}

	// cache de terceros (tutor y empresa) para no pedirlos repetido
	terceroCache := map[int]map[string]interface{}{}

	// cache de tutor->empresa_id (vía CRUD)
	empresaIdCache := map[int]int{}

	for _, it := range items {
		tutorID, _ := normalizeToInt(it["tutor_id"])
		if tutorID <= 0 {
			continue
		}

		// --- Tutor: nombre ---
		if terceroCache[tutorID] == nil {
			if data, err := getTerceroByID(ctx, tutorID); err == nil && data != nil {
				terceroCache[tutorID] = data
			}
		}
		if t := terceroCache[tutorID]; t != nil {
			it["tercero_id"] = tutorID
			nombre := strings.TrimSpace(fmt.Sprint(t["NombreCompleto"]))
			if nombre == "" {
				nombre = strings.TrimSpace(fmt.Sprint(t["nombre_completo"]))
			}
			it["nombre"] = nombre

		}

		// --- Empresa: obtener empresa_id activa del tutor (CRUD) ---
		empresaID := empresaIdCache[tutorID]
		if empresaID == 0 {
			if eid, err := getEmpresaIDActivaByTutor(ctx, tutorID); err == nil && eid > 0 {
				empresaID = eid
				empresaIdCache[tutorID] = eid
			}
		}
		if empresaID > 0 {
			it["empresa_id"] = empresaID

			// Empresa: nombre (mismo endpoint de terceros/tutor/:id)
			if terceroCache[empresaID] == nil {
				if data, err := getTerceroByID(ctx, empresaID); err == nil && data != nil {
					terceroCache[empresaID] = data
				}
			}
			if e := terceroCache[empresaID]; e != nil {
				it["empresa"] = strings.TrimSpace(fmt.Sprint(e["NombreCompleto"]))
			} else {
				it["empresa"] = fmt.Sprintf("Empresa #%d", empresaID)
			}
		}

		// Normalizamos el campo fecha para UI: usar ultima_visita como "fecha"
		if it["fecha"] == nil {
			if uv := it["ultima_visita"]; uv != nil {
				it["fecha"] = uv
			}
		}
	}
}

func getTerceroByID(ctx context.Context, id int) (map[string]interface{}, error) {
	if id <= 0 {
		return nil, nil
	}

	m, err := ObtenerTerceroPorIDCoreStd(ctx, id)
	if err != nil || m == nil {
		return nil, err
	}

	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out, nil
}

func getEmpresaIDActivaByTutor(ctx context.Context, tutorID int) (int, error) {
	if tutorID <= 0 {
		return 0, nil
	}

	cfg := rootservices.GetConfig()
	endpoint := rootservices.BuildURL(cfg.CastorCRUDBaseURL, "tutor_empresa", "activa", fmt.Sprint(tutorID))
	fmt.Println("URL EMPRESA ACTIVA", endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: cfg.RequestTimeout}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}

	// Manejo 404 como "sin empresa activa"
	if res.StatusCode == http.StatusNotFound {
		fmt.Println("EMPRESA ACTIVA 404 (sin empresa). Body:", string(bodyBytes))
		return 0, nil
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		fmt.Println("EMPRESA ACTIVA status:", res.StatusCode, "Body:", string(bodyBytes))
		return 0, fmt.Errorf("error consultando empresa activa: status=%d", res.StatusCode)
	}

	// CRUD: { success,status,message,data:{ tutor_id, empresa_id, ... } }
	var resp struct {
		Success bool   `json:"success"`
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			EmpresaID int `json:"empresa_id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		fmt.Println("EMPRESA ACTIVA unmarshal error:", err, "Body:", string(bodyBytes))
		return 0, err
	}

	fmt.Println("RESPUESTA CONSULTA EMPRESA ACTIVA (decoded)", resp)

	if resp.Data.EmpresaID > 0 {
		return resp.Data.EmpresaID, nil
	}

	// Fallback por si algún día cambia el wrapper:
	var raw map[string]any
	if err := json.Unmarshal(bodyBytes, &raw); err == nil {
		if d, ok := raw["data"].(map[string]any); ok {
			if v, ok := d["empresa_id"]; ok {
				if id, ok2 := normalizeToInt(v); ok2 && id > 0 {
					return id, nil
				}
			}
		}
	}

	return 0, nil
}

func mapPostCountsToNamedChips(counts map[string]int) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(counts))
	if len(counts) == 0 {
		return items
	}

	codes := make([]string, 0, len(counts))
	for code := range counts {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code != "" {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)

	for _, code := range codes {
		total := counts[code]
		nombre := resolveParametroNombre(code, code)

		items = append(items, map[string]interface{}{
			"estado": nombre, // 👈 esto es lo que pinta el chip
			"total":  total,
			"code":   code, // 👈 opcional, por si quieres filtrar
		})
	}

	return items
}

func buildOfertasRecomendadas(ctx context.Context, estudianteID int, pcID int) []map[string]interface{} {
	if pcID <= 0 || estudianteID <= 0 {
		return []map[string]interface{}{}
	}

	// set de oferta_id ya postuladas
	postuladas := make(map[int64]struct{})
	if list, err := clients.CastorCRUD().ListPostulaciones(ctx, map[string]string{
		"EstudianteId": fmt.Sprint(estudianteID),
	}); err == nil {
		for _, p := range list {
			if p.OfertaId > 0 {
				postuladas[p.OfertaId] = struct{}{}
			}
		}
	}

	filters := map[string]string{
		"estado": models.OfertaEstadoCreada, // "abierta" según el alias
	}
	ofertas, err := rootservices.ListOfertas(filters)
	if err != nil || len(ofertas) == 0 {
		return []map[string]interface{}{}
	}

	result := make([]map[string]interface{}, 0, recommendedOffersLimit)
	for _, oferta := range ofertas {
		// excluir si ya postuló
		if _, ok := postuladas[oferta.Id]; ok {
			continue
		}

		pcIDs, err := getPCIDsByOferta(int(oferta.Id))
		if err != nil {
			continue
		}
		if len(pcIDs) > 0 && !hasInt(pcIDs, pcID) {
			continue
		}

		entry := mapOferta(oferta)
		entry["proyecto_curricular_ids"] = pcIDs
		result = append(result, entry)

		if len(result) >= recommendedOffersLimit {
			break
		}
	}
	return result
}

func translateEstado(code string) map[string]string {
	c := strings.ToUpper(strings.TrimSpace(code))
	nombre := c
	if c != "" {
		if par, err := internalhelpers.GetParametroByCodeNoCache(c); err == nil {
			if n := strings.TrimSpace(par.Nombre); n != "" {
				nombre = n
			}
		}
	}
	if nombre == c && c == "PSRJ_CTR" {
		nombre = "Descartada"
	}
	return map[string]string{
		"code":   c,
		"nombre": nombre,
	}
}

func parseTime(value string) time.Time {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t
	}
	return time.Time{}
}

func hasInt(list []int, target int) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

// ---------- helpers: map counts -> chips con nombre ----------

func mapInvCountsToNamedChips(counts map[string]int) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(counts))
	if len(counts) == 0 {
		return items
	}

	codes := make([]string, 0, len(counts))
	for raw := range counts {
		raw = strings.ToUpper(strings.TrimSpace(raw))
		if raw != "" {
			codes = append(codes, raw)
		}
	}
	sort.Strings(codes)

	for _, raw := range codes {
		total := counts[raw]

		// Si en CRUD viene ENVIADA/ACEPTADA/RECHAZADA/EXPIRADA,
		// mapear al código de parámetros INV_*_CTR si existe.
		code := mapInvEstadoToParamCode(raw)

		nombre := resolveParametroNombre(code, raw)

		items = append(items, map[string]interface{}{
			"estado": nombre,
			"total":  total,
			"code":   code,
			"raw":    raw, // opcional para debug
		})
	}

	return items
}

func resolveParametroNombre(code string, fallback string) string {
	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "" {
		return strings.TrimSpace(fallback)
	}
	if par, err := internalhelpers.GetParametroByCodeNoCache(c); err == nil {
		if n := strings.TrimSpace(par.Nombre); n != "" {
			return n
		}
	}
	return strings.TrimSpace(fallback)
}

func mapInvEstadoToParamCode(raw string) string {
	r := strings.ToUpper(strings.TrimSpace(raw))
	switch r {
	case "ENVIADA":
		return "INV_ENV_CTR"
	case "ACEPTADA":
		return "INV_ACE_CTR"
	case "RECHAZADA":
		return "INV_REC_CTR"
	case "EXPIRADA":
		return "INV_EXP_CTR"
	case "CANCELADA":
		return "INV_CAN_CTR"
	default:
		// Si ya venía como INV_*_CTR o algún code propio, lo respeta
		return r
	}
}
func buildOfertasDisponiblesResumen(ctx context.Context, estudianteID int, pcID int) (int, []map[string]interface{}) {
	if pcID <= 0 || estudianteID <= 0 {
		return 0, []map[string]interface{}{}
	}

	// 1) Cargar postulaciones del estudiante para excluir ofertas ya postuladas
	crud := clients.CastorCRUD()
	posts, err := crud.ListPostulaciones(ctx, map[string]string{
		"EstudianteId": fmt.Sprint(estudianteID),
	})
	postuladas := make(map[int64]struct{}, len(posts))
	if err == nil {
		for _, p := range posts {
			if p.OfertaId > 0 {
				postuladas[p.OfertaId] = struct{}{}
			}
		}
	}

	// 2) Traer ofertas "abiertas" (según tu modelo: creada/publicada)
	filters := map[string]string{
		"estado": models.OfertaEstadoCreada,
	}
	ofertas, err := rootservices.ListOfertas(filters)
	if err != nil || len(ofertas) == 0 {
		return 0, []map[string]interface{}{}
	}

	// 3) Filtrar por PC y excluir postuladas
	countsByEstado := map[string]int{}
	total := 0

	for _, oferta := range ofertas {
		// Excluir si ya está postulada por el estudiante
		if _, ok := postuladas[oferta.Id]; ok {
			continue
		}

		// Filtro por proyecto curricular (oferta -> oferta_proyecto_curricular)
		pcIDs, err := getPCIDsByOferta(int(oferta.Id))
		if err != nil {
			continue
		}
		if len(pcIDs) > 0 && !hasInt(pcIDs, pcID) {
			continue
		}

		total++
		code := strings.ToUpper(strings.TrimSpace(oferta.Estado))
		if code == "" {
			code = strings.ToUpper(strings.TrimSpace(models.OfertaEstadoCreada))
		}
		countsByEstado[code]++
	}

	// 4) Convertir a chips con nombre
	chips := make([]map[string]interface{}, 0, len(countsByEstado))
	codes := make([]string, 0, len(countsByEstado))
	for c := range countsByEstado {
		codes = append(codes, c)
	}
	sort.Strings(codes)

	for _, code := range codes {
		chips = append(chips, map[string]interface{}{
			"estado": resolveParametroNombre(code, code),
			"total":  countsByEstado[code],
			"code":   code,
		})
	}

	return total, chips
}
