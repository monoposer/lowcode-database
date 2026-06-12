package version

import "testing"

func TestMap(t *testing.T) {
	m := Map()
	if m["version"] != Version {
		t.Fatalf("version=%q", m["version"])
	}
}
