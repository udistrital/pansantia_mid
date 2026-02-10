package services

// AddOASAuth agrega el header Authorization si el token está configurado.
func AddOASAuth(headers map[string]string) map[string]string {
	if headers == nil {
		headers = make(map[string]string)
	}
	token := GetConfig().OASBearerToken
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return headers
}
