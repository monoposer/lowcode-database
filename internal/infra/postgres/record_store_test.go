package postgres

import "testing"

func TestParseRecordStore(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"", RecordStoreShared, true},
		{"shared", RecordStoreShared, true},
		{"SHARED", RecordStoreShared, true},
		{"dedicated", RecordStoreDedicated, true},
		{"dedicated_table", RecordStoreDedicated, true},
		{"exclusive", RecordStoreDedicated, true},
		{"schema", "", false},
	}
	for _, c := range cases {
		got, err := ParseRecordStore(c.in)
		if c.ok {
			if err != nil || got != c.want {
				t.Fatalf("ParseRecordStore(%q)=%q %v want %q", c.in, got, err, c.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("ParseRecordStore(%q) expected error", c.in)
		}
	}
}

func TestResolveDataTables(t *testing.T) {
	t.Parallel()
	shared := ResolveDataTables("acme", "shared")
	if !shared.Shared() || shared.Record != "record" || shared.LinkRef != "link_ref" {
		t.Fatalf("shared: %+v", shared)
	}
	d := ResolveDataTables("acme", "dedicated")
	if d.Shared() || d.Record != "acme_record" || d.LinkRef != "acme_link_ref" || d.CalcQueue != "acme_calc_queue" {
		t.Fatalf("dedicated: %+v", d)
	}
	hyphen := ResolveDataTables("shop-001", RecordStoreDedicated)
	if hyphen.Record != "shop_001_record" {
		t.Fatalf("hyphen prefix: %s", hyphen.Record)
	}
	digit := ResolveDataTables("9acme", RecordStoreDedicated)
	if digit.Record != "t_9acme_record" {
		t.Fatalf("digit prefix: %s", digit.Record)
	}
}
