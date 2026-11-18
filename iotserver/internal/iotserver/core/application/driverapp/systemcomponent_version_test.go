package driverapp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeEKuiperVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"version":"2.4.2","os":"linux"}`)
	}))
	defer srv.Close()

	if got := probeEKuiperVersion(context.Background(), srv.URL); got != "2.4.2" {
		t.Fatalf("ekuiper version = %q", got)
	}
}

// /api/v1/version 无需鉴权，Token 为空时也应当能拿到版本。
func TestProbeIotEdgeDBVersionFromPublicEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/version" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("version endpoint should not need auth, got %q", got)
		}
		_, _ = io.WriteString(w, `{"version":"0.10.0","date":"2026-11-13"}`)
	}))
	defer srv.Close()

	if got := probeIotEdgeDBVersion(context.Background(), srv.URL, ""); got != "0.10.0" {
		t.Fatalf("iotedgedb version = %q", got)
	}
}

// 老版本没有 /api/v1/version 时，仍用 token 读 /api/v1/status 兜底。
func TestProbeIotEdgeDBVersionFallsBackToStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/version":
			http.NotFound(w, r)
		case "/api/v1/status":
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Fatalf("authorization = %q", got)
			}
			_, _ = io.WriteString(w, `{"version":"0.9.0","uptime_secs":1}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	if got := probeIotEdgeDBVersion(context.Background(), srv.URL, "test-token"); got != "0.9.0" {
		t.Fatalf("iotedgedb fallback version = %q", got)
	}
}

func TestTagFromImage(t *testing.T) {
	cases := []struct {
		image string
		want  string
	}{
		{"mqtt-broker:2.6", "2.6"},
		{"registry.cn-shanghai.aliyuncs.com/winc-link/mqtt-broker:2.6", "2.6"},
		{"registry:5000/team/engine:0.3.1", "0.3.1"},
		{"registry:5000/team/engine", ""},
		{"mqtt-broker", ""},
		{"mqtt-broker:<none>", ""},
		{"mqtt-broker@sha256:abcdef", ""},
		{"sha256:abcdef0123456789", ""},
		{"sha512:abcdef0123456789", ""},
	}
	for _, tc := range cases {
		if got := tagFromImage(tc.image); got != tc.want {
			t.Fatalf("tagFromImage(%q) = %q, want %q", tc.image, got, tc.want)
		}
	}
}

func TestEndpointPort(t *testing.T) {
	if got := endpointPort("http://192.168.0.74:8081"); got != "8081" {
		t.Fatalf("endpointPort = %q", got)
	}
	if got := endpointPort("192.168.0.74:8081"); got != "8081" {
		t.Fatalf("endpointPort without scheme = %q", got)
	}
	if got := endpointPort("http://engine"); got != "" {
		t.Fatalf("endpointPort no port = %q", got)
	}
}
