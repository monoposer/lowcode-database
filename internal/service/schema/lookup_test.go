package schema

import (
	"testing"

	"github.com/solat/lowcode-database/internal/service/shared"
)

func TestLookupTargetAllowed(t *testing.T) {
	for _, kind := range []string{"text", "formula", "lookup", "rollup"} {
		if !shared.LookupTargetAllowed(kind) {
			t.Fatalf("%s should be allowed", kind)
		}
	}
	if shared.LookupTargetAllowed("relationship") {
		t.Fatal("relationship should not be a lookup target")
	}
}
