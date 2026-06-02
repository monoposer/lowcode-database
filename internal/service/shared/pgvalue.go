package shared

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/solat/lowcode-database/internal/apiv1"
)

func isUUIDPgType(pgType string) bool {
	pgType = strings.ToLower(strings.TrimSpace(pgType))
	if pgType == "uuid" {
		return true
	}
	if i := strings.LastIndex(pgType, "."); i >= 0 {
		return strings.ToLower(pgType[i+1:]) == "uuid"
	}
	return false
}

func isByteaPgType(pgType string) bool {
	pgType = strings.ToLower(strings.TrimSpace(pgType))
	return pgType == "bytea"
}

// FormatUUID converts pgx UUID wire forms to canonical string.
func FormatUUID(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		if t == "" {
			return "", false
		}
		if u, err := uuid.Parse(t); err == nil {
			return u.String(), true
		}
		return t, true
	case []byte:
		if len(t) == 16 {
			u, err := uuid.FromBytes(t)
			if err == nil {
				return u.String(), true
			}
		}
	case [16]byte:
		u, err := uuid.FromBytes(t[:])
		if err == nil {
			return u.String(), true
		}
	case pgtype.UUID:
		if !t.Valid {
			return "", false
		}
		u, err := uuid.FromBytes(t.Bytes[:])
		if err == nil {
			return u.String(), true
		}
	case *pgtype.UUID:
		if t == nil || !t.Valid {
			return "", false
		}
		u, err := uuid.FromBytes(t.Bytes[:])
		if err == nil {
			return u.String(), true
		}
	}
	return "", false
}

// PGValueToNative converts a PostgreSQL driver value to plain JSON scalars.
func PGValueToNative(v any, pgType string) any {
	if v == nil {
		return nil
	}
	if isUUIDPgType(pgType) {
		if s, ok := FormatUUID(v); ok {
			return s
		}
	}
	if isByteaPgType(pgType) {
		switch t := v.(type) {
		case []byte:
			return base64.StdEncoding.EncodeToString(t)
		}
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		if len(t) == 16 {
			if s, ok := FormatUUID(t); ok {
				return s
			}
		}
		return base64.StdEncoding.EncodeToString(t)
	case bool:
		return t
	case time.Time:
		return t.Format(time.RFC3339Nano)
	case int32, int64, float32, float64:
		return toFloat64(t)
	case pgtype.Numeric:
		if f, err := numericToFloat64(t); err == nil {
			return f
		}
	case *pgtype.Numeric:
		if t != nil {
			if f, err := numericToFloat64(*t); err == nil {
				return f
			}
		}
	case []string:
		out := make([]any, len(t))
		for i, s := range t {
			out[i] = s
		}
		return out
	case []any:
		return t
	default:
		return fmt.Sprint(v)
	}
	return fmt.Sprint(v)
}

// DBCellValue converts a scanned PG value into an API cell Value.
func DBCellValue(v any, pgType string) *apiv1.Value {
	return apiv1.NativeToValue(PGValueToNative(v, pgType))
}
