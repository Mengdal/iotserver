package iotp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuditLogs(t *testing.T) {
	var gotPath, gotLimit, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotLimit = r.URL.Query().Get("limit")
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"timestamp":"2026-11-16T10:00:00Z","actor":"admin","event":"device_added","target":"db.t.dev"}]}`))
	}))
	defer srv.Close()

	client := New(srv.URL, "tok", "iotserver", time.Second)
	entries, err := client.AuditLogs(context.Background(), 3)
	if err != nil {
		t.Fatalf("audit logs: %v", err)
	}
	if gotPath != "/api/v1/meta/auth/audit" || gotLimit != "3" || gotAuth != "Bearer tok" {
		t.Fatalf("unexpected request path=%q limit=%q auth=%q", gotPath, gotLimit, gotAuth)
	}
	if len(entries) != 1 || entries[0]["actor"] != "admin" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

// 旧版本可能直接返回数组。
func TestAuditLogsPlainArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"actor":"cli"},{"actor":"api"}]`))
	}))
	defer srv.Close()

	entries, err := New(srv.URL, "", "iotserver", time.Second).AuditLogs(context.Background(), 10)
	if err != nil {
		t.Fatalf("audit logs: %v", err)
	}
	if len(entries) != 2 || entries[0]["actor"] != "cli" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestAuditLogsUnknownShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"unexpected":true}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "", "iotserver", time.Second).AuditLogs(context.Background(), 10); err == nil {
		t.Fatal("unexpected response shape should fail")
	}
}
