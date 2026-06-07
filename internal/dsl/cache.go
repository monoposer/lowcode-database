package dsl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
)

type parseEntry struct {
	w   Where
	err error
}

var parseCache sync.Map // string -> parseEntry

const parseCacheMax = 4096

var parseCacheSize int
var parseCacheMu sync.Mutex

// ParseCached is Parse with a process-local cache keyed by canonical JSON.
func ParseCached(raw any) (Where, error) {
	if raw == nil {
		return Where{}, nil
	}
	key, err := filterCacheKey(raw)
	if err != nil {
		return Parse(raw)
	}
	if v, ok := parseCache.Load(key); ok {
		e := v.(parseEntry)
		return e.w, e.err
	}
	w, err := Parse(raw)
	parseCacheMu.Lock()
	if parseCacheSize < parseCacheMax {
		parseCache.Store(key, parseEntry{w: w, err: err})
		parseCacheSize++
	}
	parseCacheMu.Unlock()
	return w, err
}

func filterCacheKey(raw any) (string, error) {
	switch v := raw.(type) {
	case string:
		sum := sha256.Sum256([]byte(v))
		return hex.EncodeToString(sum[:]), nil
	case []byte:
		sum := sha256.Sum256(v)
		return hex.EncodeToString(sum[:]), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(b)
		return hex.EncodeToString(sum[:]), nil
	}
}
