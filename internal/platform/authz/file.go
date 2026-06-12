package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// -------- Types --------
// -------- Types --------

// fileConfig is the on-disk RBAC definition.
//
// Example:
//
//	{
//	  "roles": {
//	    "admin": ["*:*", "schema:*", "platform:*"],
//	    "viewer": ["*:select"],
//	    "editor": ["*:select", "*:insert", "*:update", "*:delete"]
//	  }
//	}
//
// Permission format: "<resource>:<action>"
//   - resource: table name, "*", "schema", or "platform"
//   - action: select|insert|update|delete|read|write|*
type fileConfig struct {
	Roles map[string][]string `json:"roles"`
}

// FileAuthorizer checks role → permission grants loaded from a JSON file.
type FileAuthorizer struct {
	path    string
	mu      sync.RWMutex
	cfg     fileConfig
	lastMod time.Time
}

func NewFileAuthorizer(path string) (*FileAuthorizer, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("AUTHZ_FILE is required when AUTHZ_DRIVER=file")
	}
	a := &FileAuthorizer{path: path}
	if err := a.reload(); err != nil {
		return nil, err
	}
	return a, nil
}

// -------- Allow --------

func (a *FileAuthorizer) Allow(_ context.Context, req Request) (bool, error) {
	if err := a.reloadIfChanged(); err != nil {
		return false, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()

	resource := resourceKey(req.Resource)
	action := string(req.Action)
	if req.Resource.Type != "db" {
		switch req.Action {
		case ActionSelect:
			action = "read"
		case ActionInsert, ActionUpdate, ActionDelete:
			action = "write"
		}
	}

	for _, role := range req.User.Roles {
		for _, perm := range a.cfg.Roles[role] {
			if matchPermission(perm, resource, action) {
				return true, nil
			}
		}
	}
	return false, nil
}

func resourceKey(res Resource) string {
	switch res.Type {
	case "db":
		return res.Table
	case "schema", "platform":
		return res.Type
	default:
		return res.Type
	}
}

func matchPermission(perm, resource, action string) bool {
	parts := strings.SplitN(strings.TrimSpace(perm), ":", 2)
	if len(parts) != 2 {
		return false
	}
	pr, pa := parts[0], parts[1]
	if pr == "*" && pa == "*" {
		return true
	}
	if pr == resource && pa == "*" {
		return true
	}
	if pr == "*" && pa == action {
		return true
	}
	if pr == resource && pa == action {
		return true
	}
	return false
}

// -------- Reload --------

func (a *FileAuthorizer) reloadIfChanged() error {
	a.mu.RLock()
	path := a.path
	a.mu.RUnlock()
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("authz file: %w", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.Roles != nil && !info.ModTime().After(a.lastMod) {
		return nil
	}
	return a.reloadLocked()
}

func (a *FileAuthorizer) reload() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.reloadLocked()
}

func (a *FileAuthorizer) reloadLocked() error {
	b, err := os.ReadFile(a.path)
	if err != nil {
		return fmt.Errorf("read authz file: %w", err)
	}
	var cfg fileConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return fmt.Errorf("parse authz file: %w", err)
	}
	if cfg.Roles == nil {
		cfg.Roles = map[string][]string{}
	}
	info, err := os.Stat(a.path)
	if err != nil {
		return err
	}
	a.cfg = cfg
	a.lastMod = info.ModTime()
	return nil
}
