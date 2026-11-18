package ekuiperclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 已存在时只 GET，不再 POST，避免 eKuiper 打 "already exists" 的 error 日志。
func TestEnsureStreamSkipsExisting(t *testing.T) {
	posted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/streams/"+MQTTStreamName:
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/streams":
			posted = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	created, err := EnsureStream(context.Background(), srv.URL, nil, MQTTStreamName, MQTTStreamSQL)
	if err != nil {
		t.Fatalf("EnsureStream error: %v", err)
	}
	if created {
		t.Fatal("stream already exists, created should be false")
	}
	if posted {
		t.Fatal("stream already exists, should not POST /streams")
	}
}

func TestEnsureStreamCreatesWhenMissing(t *testing.T) {
	var sql string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/streams":
			body, _ := io.ReadAll(r.Body)
			sql = string(body)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	created, err := EnsureStream(context.Background(), srv.URL, nil, MQTTStreamName, MQTTStreamSQL)
	if err != nil {
		t.Fatalf("EnsureStream error: %v", err)
	}
	if !created {
		t.Fatal("stream missing, created should be true")
	}
	if !strings.Contains(sql, "CREATE STREAM mqtt_stream") {
		t.Fatalf("unexpected create payload: %s", sql)
	}
}

// eKuiper 2.4.2 对缺失流返回 400 + "is not found"，应继续创建而不是报错。
func TestEnsureStreamCreatesWhenGetReturns400NotFound(t *testing.T) {
	created := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":3000,"message":"describe stream error: does_not_exist is not found."}`)
		case r.Method == http.MethodPost && r.URL.Path == "/streams":
			created = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	ok, err := EnsureStream(context.Background(), srv.URL, nil, MQTTStreamName, MQTTStreamSQL)
	if err != nil {
		t.Fatalf("400 not found should not fail: %v", err)
	}
	if !ok || !created {
		t.Fatalf("stream should be created, ok=%v created=%v", ok, created)
	}
}

// 查询接口的其它错误不能被吞掉，否则真正的故障会被后续创建错误覆盖。
func TestEnsureStreamReturnsGetError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer srv.Close()

	if _, err := EnsureStream(context.Background(), srv.URL, nil, MQTTStreamName, MQTTStreamSQL); err == nil {
		t.Fatal("GET 500 should return an error")
	}
}

func TestEnsureStreamTreatsRaceAsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"Create stream fails: Item mqtt_stream already exists."}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	created, err := EnsureStream(context.Background(), srv.URL, nil, MQTTStreamName, MQTTStreamSQL)
	if err != nil {
		t.Fatalf("already exists should not be an error: %v", err)
	}
	if created {
		t.Fatal("already exists should report created=false")
	}
}
