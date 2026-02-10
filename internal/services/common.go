package services

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	beegocontext "github.com/beego/beego/v2/server/web/context"
)

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func normalizeToInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		if parsed, err := strconv.Atoi(v.String()); err == nil {
			return parsed, true
		}
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func normalizeToInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case json.Number:
		if parsed, err := strconv.ParseInt(v.String(), 10, 64); err == nil {
			return parsed, true
		}
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
		if parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func normalizeToBool(value interface{}, def bool) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			lower := strings.ToLower(trimmed)
			return lower == "true" || lower == "1" || lower == "si"
		}
	case float64:
		return v != 0
	case int:
		return v != 0
	}
	return def
}

func normalizeToString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.Itoa(int(v))
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func requestContext(ctx *beegocontext.Context) stdctx.Context {
	if ctx != nil && ctx.Request != nil {
		return ctx.Request.Context()
	}
	return stdctx.Background()
}
