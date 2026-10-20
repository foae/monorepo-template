package envutil

import (
	"strings"
	"testing"
)

func TestReplaceHost(t *testing.T) {
	// Environment variables are process-global; these cases must not run in parallel.
	for _, tc := range []struct {
		name, override, value, want string
		wantErr                     bool
	}{
		{name: "unset", value: "unchanged", want: "unchanged"},
		{name: "hostname", override: "db.internal", value: "postgres://user:secret@old:5432/db?sslmode=require", want: "postgres://user:secret@db.internal:5432/db?sslmode=require"},
		{name: "ignored port", override: "db:9999", value: "postgres://old:5432/db", want: "postgres://db:5432/db"},
		{name: "container alias", override: "db_primary", value: "postgres://old_db:5432/db", want: "postgres://db_primary:5432/db"},
		{name: "bare IPv6", override: "2001:db8::1234", value: "postgres://old:5432/db", want: "postgres://[2001:db8::1234]:5432/db"},
		{name: "bracketed IPv6", override: "[::1]", value: "http://old/path", want: "http://[::1]/path"},
		{name: "IPv6 port", override: "[::1]:9999", value: "http://old:80/path", want: "http://[::1]:80/path"},
		{name: "scoped IPv6", override: "fe80::1%eth0", value: "http://old/path", want: "http://[fe80::1%25eth0]/path"},
		{name: "scoped source", override: "[fe80::2%eth1]", value: "http://[fe80::1%25eth0]:8080/path", want: "http://[fe80::2%25eth1]:8080/path"},
		{name: "override URL", override: "https://secret@db", value: "http://old", wantErr: true},
		{name: "override path", override: "db/secret", value: "http://old", wantErr: true},
		{name: "override query", override: "db?secret", value: "http://old", wantErr: true},
		{name: "override whitespace", override: "db secret", value: "http://old", wantErr: true},
		{name: "override port range", override: "db:65536", value: "http://old", wantErr: true},
		{name: "override empty port", override: "db:", value: "http://old", wantErr: true},
		{name: "relative URL", override: "db", value: "secret/path", wantErr: true},
		{name: "missing URL host", override: "db", value: "postgres:///secret", wantErr: true},
		{name: "invalid URL", override: "db", value: "postgres://user:secret@old:invalid/db", wantErr: true},
		{name: "URL port range", override: "db", value: "postgres://user:secret@old:99999/db", wantErr: true},
		{name: "unbracketed source IPv6", override: "db", value: "http://2001:db8::1/db", wantErr: true},
		{name: "unsafe IPv6 scope", override: "fe80::1%secret/path", value: "http://old", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEST_DATABASE_HOST", tc.override)
			got, err := ReplaceHost("TEST_DATABASE", tc.value)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want error %v", err, tc.wantErr)
			}
			if err != nil {
				if strings.Contains(err.Error(), "secret") || got != "" {
					t.Fatal("configuration leaked on error")
				}
				return
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
