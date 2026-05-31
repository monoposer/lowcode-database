package shared

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func IsNumericPgType(pg string) bool {
	switch pg {
	case "bigint", "integer", "smallint", "double precision", "real":
		return true
	default:
		return strings.HasPrefix(pg, "numeric")
	}
}

func IsTextLikePgType(pg string) bool {
	return pg == "text" || pg == "varchar" || strings.HasPrefix(pg, "character varying")
}

// ColumnAlterTypeUsing builds a PostgreSQL USING expression for ALTER COLUMN TYPE.
func ColumnAlterTypeUsing(fromPg, toPg, pgColumn string) string {
	col := pgx.Identifier{pgColumn}.Sanitize()

	if IsTextLikePgType(fromPg) && IsNumericPgType(toPg) {
		if toPg == "double precision" || toPg == "real" || strings.HasPrefix(toPg, "numeric") {
			pattern := `'^-?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][-+]?[0-9]+)?$'`
			return fmt.Sprintf(
				`CASE WHEN %s IS NULL OR btrim(%s::text) = '' THEN NULL WHEN btrim(%s::text) ~ %s THEN btrim(%s::text)::%s ELSE NULL END`,
				col, col, col, pattern, col, toPg,
			)
		}
		return fmt.Sprintf(
			`CASE WHEN %s IS NULL OR btrim(%s::text) = '' THEN NULL WHEN btrim(%s::text) ~ '^-?[0-9]+$' THEN btrim(%s::text)::%s ELSE NULL END`,
			col, col, col, col, toPg,
		)
	}

	if IsTextLikePgType(fromPg) && toPg == "boolean" {
		return fmt.Sprintf(
			`CASE WHEN %s IS NULL OR btrim(%s::text) = '' THEN NULL WHEN lower(btrim(%s::text)) IN ('true','t','1','yes','y') THEN true WHEN lower(btrim(%s::text)) IN ('false','f','0','no','n') THEN false ELSE NULL END`,
			col, col, col, col,
		)
	}

	if IsNumericPgType(fromPg) && toPg == "boolean" {
		return fmt.Sprintf(`CASE WHEN %s IS NULL THEN NULL ELSE (%s <> 0) END`, col, col)
	}

	if toPg == "text" {
		return fmt.Sprintf(`%s::text`, col)
	}

	return fmt.Sprintf(`%s::%s`, col, toPg)
}
