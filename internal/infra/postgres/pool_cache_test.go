package postgres

import "testing"

func TestDSNPoolKeyReuse(t *testing.T) {
	a := dsnPoolKey(" postgresql://u:p@h/db ")
	b := dsnPoolKey("postgresql://u:p@h/db")
	if a != b {
		t.Fatalf("keys differ: %q %q", a, b)
	}
}

func TestDSNFingerprintNoPassword(t *testing.T) {
	fp := DSNFingerprint("postgresql://user:secret@localhost:5432/lowcode_data")
	if fp == "" {
		t.Fatal("empty fingerprint")
	}
	if contains(fp, "secret") || contains(fp, "user") {
		t.Fatalf("fingerprint leaked credentials: %s", fp)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && (s == sub || len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}
