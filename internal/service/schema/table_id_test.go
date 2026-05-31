package schema

import "testing"

func TestResolveTableIDTypeID(t *testing.T) {
	id, err := resolveTableIDTypeID("")
	if err != nil || id != "uuid" {
		t.Fatalf("default: got %q err=%v", id, err)
	}
	id, err = resolveTableIDTypeID("int8")
	if err != nil || id != "int8" {
		t.Fatalf("int8: got %q err=%v", id, err)
	}
	id, err = resolveTableIDTypeID("text")
	if err != nil || id != "text" {
		t.Fatalf("text: got %q err=%v", id, err)
	}
	if _, err := resolveTableIDTypeID("formula"); err == nil {
		t.Fatal("expected error for virtual idType")
	}
	if _, err := resolveTableIDTypeID("nope"); err == nil {
		t.Fatal("expected error for unknown idType")
	}
}

func TestBuildTableIDColumnDDL(t *testing.T) {
	u, err := buildTableIDColumnDDL("uuid")
	if err != nil || u == "" {
		t.Fatalf("uuid ddl: %q err=%v", u, err)
	}
	i, err := buildTableIDColumnDDL("int8")
	if err != nil || i == "" {
		t.Fatalf("int8 ddl: %q err=%v", i, err)
	}
	if u == i {
		t.Fatal("uuid and int8 DDL should differ")
	}
	txt, err := buildTableIDColumnDDL("text")
	if err != nil || txt == "" {
		t.Fatalf("text ddl: %q err=%v", txt, err)
	}
}

func TestPgTypeToLogicalID(t *testing.T) {
	if pgTypeToLogicalID("bigint") != "int8" {
		t.Fatal("bigint")
	}
	if pgTypeToLogicalID("uuid") != "uuid" {
		t.Fatal("uuid")
	}
}

func TestPageTokenIDCompare(t *testing.T) {
	if PageTokenIDCompare("_b", "bigint", 1) == PageTokenIDCompare("_b", "uuid", 1) {
		t.Fatal("bigint vs uuid compare should differ")
	}
}
