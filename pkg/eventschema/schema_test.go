package eventschema

import (
	"encoding/json"
	"testing"
)

func TestEnvelopeSchemaValidJSON(t *testing.T) {
	var v any
	if err := json.Unmarshal(EnvelopeSchema(), &v); err != nil {
		t.Fatal(err)
	}
}

func TestAllPayloadSchemas(t *testing.T) {
	for _, typ := range PayloadSchemaTypes() {
		raw, ok := PayloadSchema(typ)
		if !ok {
			t.Fatalf("missing schema for %s", typ)
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
	}
}

func TestValidType(t *testing.T) {
	if !ValidType(RecordsAfterInsert) {
		t.Fatal("expected valid")
	}
	if ValidType("nope") {
		t.Fatal("expected invalid")
	}
	if CategoryOf(MetadataTableCreated) != CategoryMetadata {
		t.Fatal("expected metadata category")
	}
}
