package iotp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientLifecycle(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("authorization = %q", got)
		}
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/databases":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"database exists","code":"CONFLICT"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/tables":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"table exists","code":"CONFLICT"}`))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/fields"):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/devices":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"device exists","code":"CONFLICT"}`))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/v1/meta/devices/"):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"device not found","code":"NOT_FOUND"}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	client := New(srv.URL, "tok", "iotserver", 2*time.Second)
	ctx := context.Background()
	if err := client.EnsureDatabase(ctx); err != nil {
		t.Fatalf("EnsureDatabase: %v", err)
	}
	fields := []Field{
		{Name: "tenant_id", Type: TypeString, IsTag: true},
		{Name: "temperature", Type: TypeF64},
	}
	if err := client.EnsureTable(ctx, "p_1", fields); err != nil {
		t.Fatalf("EnsureTable: %v", err)
	}
	if err := client.RegisterDevice(ctx, "p_1", Device{
		DeviceID: 123,
		Name:     "device-a",
		Tags:     map[string]string{"tenant_id": "t1"},
	}); err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}
	if err := client.DeleteDevice(ctx, 123); err != nil {
		t.Fatalf("DeleteDevice (404 should be ok): %v", err)
	}

	mu.Lock()
	joined := strings.Join(calls, "\n")
	mu.Unlock()
	for _, want := range []string{
		"POST /api/v1/meta/databases",
		"POST /api/v1/meta/tables",
		"PUT /api/v1/meta/tables/iotserver/p_1/fields",
		"POST /api/v1/meta/devices",
		"PUT /api/v1/meta/devices/123",
		"DELETE /api/v1/meta/devices/123",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing call %q in:\n%s", want, joined)
		}
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"field type conflict","code":"CONFLICT"}`))
	}))
	defer srv.Close()
	client := New(srv.URL, "", "iotserver", time.Second)
	err := client.AddField(context.Background(), "p_1", Field{Name: "x", Type: TypeF64})
	if !IsConflict(err) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestTimeUnitConversion(t *testing.T) {
	const millis = int64(1794448335000)
	if got := MillisToNanos(millis); got != millis*1_000_000 {
		t.Fatalf("MillisToNanos(%d) = %d", millis, got)
	}
	cases := map[int64]int64{
		millis:             millis, // 毫秒
		millis * 1_000:     millis, // 微秒
		millis * 1_000_000: millis, // 纳秒
		millis / 1_000:     millis, // 秒
	}
	for in, want := range cases {
		if got := NanosToMillis(in); got != want {
			t.Fatalf("NanosToMillis(%d) = %d, want %d", in, got, want)
		}
	}
}
