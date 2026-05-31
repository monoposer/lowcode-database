package shared

import (
	"strings"
	"testing"
)

func TestRollupAggregateSQL_quotesCamelCaseTable(t *testing.T) {
	sql := RollupAggregateSQL("sum", "amount", "orderId", "public", "orderGoods", "_b", "")
	if !strings.Contains(sql, `"orderGoods"`) {
		t.Fatalf("expected quoted table name, got %q", sql)
	}
	if !strings.Contains(sql, `"orderId"`) {
		t.Fatalf("expected quoted link column, got %q", sql)
	}
	if strings.Contains(sql, " FROM public.orderGoods ") {
		t.Fatalf("must not use unquoted identifiers: %q", sql)
	}
}

func TestRollupAggregateSQL_extraWhere(t *testing.T) {
	sql := RollupAggregateSQL("count", "", "orderId", "public", "orderGoods", "_b", "_r.status = 'active'")
	if !strings.Contains(sql, "_r.status = 'active'") {
		t.Fatalf("got %q", sql)
	}
}
