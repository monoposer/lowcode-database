package query

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"

	"github.com/monoposer/lowcode-database/internal/dsl"
)

type whereCacheEntry struct {
	sql  string
	args []any
	err  error
}

var whereCache sync.Map
var whereCacheSize int
var whereCacheMu sync.Mutex

const whereCacheMax = 4096

// BuildWhereWithTypesCached caches SQL assembly for identical filter + column maps.
func BuildWhereWithTypesCached(w dsl.Where, attrToPg, attrPgTypes map[string]string, argStart int) (string, []any, error) {
	key, err := whereCacheKey(w, attrToPg, attrPgTypes, argStart)
	if err != nil {
		return BuildWhereWithTypes(w, attrToPg, attrPgTypes, argStart)
	}
	if v, ok := whereCache.Load(key); ok {
		e := v.(whereCacheEntry)
		return e.sql, append([]any(nil), e.args...), e.err
	}
	sql, args, err := BuildWhereWithTypes(w, attrToPg, attrPgTypes, argStart)
	whereCacheMu.Lock()
	if whereCacheSize < whereCacheMax {
		whereCache.Store(key, whereCacheEntry{sql: sql, args: append([]any(nil), args...), err: err})
		whereCacheSize++
	}
	whereCacheMu.Unlock()
	return sql, args, err
}

func whereCacheKey(w dsl.Where, attrToPg, attrPgTypes map[string]string, argStart int) (string, error) {
	payload := struct {
		W     dsl.Where   `json:"w"`
		A     [][2]string `json:"a"`
		T     [][2]string `json:"t"`
		Start int         `json:"s"`
	}{W: w, A: sortedPairs(attrToPg), T: sortedPairs(attrPgTypes), Start: argStart}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func sortedPairs(m map[string]string) [][2]string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([][2]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, [2]string{k, m[k]})
	}
	return out
}
