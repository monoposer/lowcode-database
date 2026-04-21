package service

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/solat/lowcode-database/internal/apiv1"
)

// GetDatabaseConnection returns host/user/db and a passwordless URL plus psql hints for the active tenant database.
func (s *LowcodeService) GetDatabaseConnection(ctx context.Context, _ *apiv1.GetDatabaseConnectionRequest) (*apiv1.GetDatabaseConnectionResponse, error) {
	dsn, err := s.tenants.EffectiveDataDSN(ctx)
	if err != nil {
		return nil, err
	}
	host, port, database, user, urlNoPass, psqlCmd, err := parsePostgresDSNForDisplay(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	hint := passwordHint()
	return &apiv1.GetDatabaseConnectionResponse{
		Host:               host,
		Port:               int32(port),
		Database:           database,
		User:               user,
		UrlWithoutPassword: urlNoPass,
		PsqlCommand:        psqlCmd,
		PasswordSourceHint: hint,
	}, nil
}

func passwordHint() string {
	return "This API never returns a password. Use the password embedded in this tenant's data_dsn (lc_tenants / DEFAULT_TENANT_DATA_DSN), or set PGPASSWORD / ~/.pgpass when running psql locally."
}

func parsePostgresDSNForDisplay(dsn string) (host string, port int, database, user, urlWithoutPassword, psqlCommand string, err error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", 0, "", "", "", "", err
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", 0, "", "", "", "", fmt.Errorf("unsupported scheme %q (expected postgres or postgresql)", u.Scheme)
	}
	host = u.Hostname()
	if host == "" {
		host = "localhost"
	}
	portStr := u.Port()
	if portStr == "" {
		port = 5432
	} else {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return "", 0, "", "", "", "", fmt.Errorf("invalid port %q", portStr)
		}
		port = p
	}
	path := strings.TrimPrefix(u.Path, "/")
	if path == "" {
		return "", 0, "", "", "", "", fmt.Errorf("dsn has empty database name")
	}
	parts := strings.Split(path, "/")
	database = parts[len(parts)-1]

	if u.User != nil {
		user = u.User.Username()
	}

	clone := *u
	if user != "" {
		clone.User = url.User(user)
	} else {
		clone.User = nil
	}
	urlWithoutPassword = clone.String()

	// Escape single quotes in psql -c style args; use separate -h -p flags instead of conninfo string.
	psqlCommand = fmt.Sprintf(`psql -h %s -p %d -U %s -d %s`, shellQuoteArg(host), port, shellQuoteArg(user), shellQuoteArg(database))
	return host, port, database, user, urlWithoutPassword, psqlCommand, nil
}

func shellQuoteArg(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, ` '"\`) {
		return s
	}
	return `'` + strings.ReplaceAll(s, `'`, `'"'"'`) + `'`
}
