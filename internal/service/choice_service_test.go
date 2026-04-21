package service

import (
	"testing"

	"github.com/solat/lowcode-database/internal/apiv1"
)

func TestChoiceLogicalNameFromPgType(t *testing.T) {
	name, err := choiceLogicalNameFromPgType("test", "lc_e_test_order_status")
	if err != nil {
		t.Fatal(err)
	}
	if name != "order_status" {
		t.Fatalf("got %q", name)
	}
}

func TestChoicePgTypeName(t *testing.T) {
	name, err := choicePgTypeName("test", "order_status")
	if err != nil {
		t.Fatal(err)
	}
	if name != "lc_e_test_order_status" {
		t.Fatalf("got %q", name)
	}
}

func TestEnumValuesFromItems(t *testing.T) {
	lits, err := enumValuesFromItems([]*apiv1.ChoiceItem{
		{Value: "active", Label: "Active"},
		{Name: "inactive", Label: "Inactive"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lits) != 2 {
		t.Fatalf("got %d literals", len(lits))
	}
}

func TestSanitizePgIdent(t *testing.T) {
	if _, err := sanitizePgIdent("Bad-Name!"); err == nil {
		t.Fatal("expected error")
	}
	s, err := sanitizePgIdent("Good_Name-1")
	if err != nil || s != "good_name_1" {
		t.Fatalf("got %q err=%v", s, err)
	}
}

func TestIndexSQLName(t *testing.T) {
	n, err := indexSQLName("vendor", "score")
	if err != nil {
		t.Fatal(err)
	}
	if n != "idx_vendor_score" {
		t.Fatalf("got %q", n)
	}
}
