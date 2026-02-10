package helpers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/beego/beego/v2/server/web/context"
)

const ctxClaimsKey = "__pasantia_mid_jwt_claims"

var (
	// ErrNoAuthHeader se devuelve cuando no se encuentra el header Authorization.
	ErrNoAuthHeader = errors.New("authorization header missing")
	// ErrInvalidToken se devuelve cuando el formato del token no es un JWT válido.
	ErrInvalidToken = errors.New("invalid bearer token")
	// ErrClaimNotFound indica que el claim requerido no está presente.
	ErrClaimNotFound = errors.New("claim not found")
)

// Claims obtiene y almacena en caché los claims del JWT presente en Authorization.
func Claims(ctx *context.Context) (map[string]interface{}, error) {
	if cached := ctx.Input.GetData(ctxClaimsKey); cached != nil {
		if claims, ok := cached.(map[string]interface{}); ok {
			return claims, nil
		}
	}

	token, err := extractBearer(ctx)
	if err != nil {
		return nil, err
	}
	claims, err := decodeClaims(token)
	if err != nil {
		return nil, err
	}
	ctx.Input.SetData(ctxClaimsKey, claims)
	return claims, nil
}

// GetTutorID retorna el claim tutor_id como entero.
func GetTutorID(ctx *context.Context) (int, error) {
	return getIntClaim(ctx, "tutor_id")
}

// GetTerceroID retorna el claim tercero_id como entero.
func GetTerceroID(ctx *context.Context) (int, error) {
	return getIntClaim(ctx, "tercero_id")
}

// RequireRole valida que el token contenga al menos uno de los roles requeridos.
func RequireRole(ctx *context.Context, roles ...string) error {
	if len(roles) == 0 {
		return nil
	}
	claims, err := Claims(ctx)
	if err != nil {
		return err
	}

	userRoles := extractRoles(claims)
	if len(userRoles) == 0 {
		return fmt.Errorf("%w: roles", ErrClaimNotFound)
	}

	roleSet := make(map[string]struct{}, len(userRoles))
	for _, r := range userRoles {
		roleSet[strings.ToLower(strings.TrimSpace(r))] = struct{}{}
	}

	for _, required := range roles {
		if _, ok := roleSet[strings.ToLower(strings.TrimSpace(required))]; ok {
			return nil
		}
	}
	return errors.New("insufficient roles")
}

func getIntClaim(ctx *context.Context, key string) (int, error) {
	claims, err := Claims(ctx)
	if err != nil {
		return 0, err
	}
	value, ok := claims[key]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrClaimNotFound, key)
	}
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, err
		}
		return int(n), nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, fmt.Errorf("%w: %s", ErrClaimNotFound, key)
		}
		n, err := json.Number(strings.TrimSpace(v)).Int64()
		if err != nil {
			return 0, err
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("claim %s is not numeric", key)
	}
}

func extractBearer(ctx *context.Context) (string, error) {
	header := strings.TrimSpace(ctx.Input.Header("Authorization"))
	if header == "" {
		return "", ErrNoAuthHeader
	}

	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return "", ErrInvalidToken
	}
	return strings.TrimSpace(header[7:]), nil
}

func decodeClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func extractRoles(claims map[string]interface{}) []string {
	if roles := parseRolesValue(claims["roles"]); len(roles) > 0 {
		return roles
	}
	if roles := parseRolesValue(claims["role"]); len(roles) > 0 {
		return roles
	}
	// Common nested structure: realm_access.roles
	if realm, ok := claims["realm_access"].(map[string]interface{}); ok {
		if roles := parseRolesValue(realm["roles"]); len(roles) > 0 {
			return roles
		}
	}
	return nil
}

func parseRolesValue(raw interface{}) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		split := strings.Split(v, ",")
		result := make([]string, 0, len(split))
		for _, part := range split {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			switch r := item.(type) {
			case string:
				if trimmed := strings.TrimSpace(r); trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
		return result
	case []string:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	default:
		return nil
	}
}
