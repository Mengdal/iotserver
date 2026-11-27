package httphelper

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// SSE 长连接需要能拿到 http.Server 的原始 ResponseWriter 并清掉 WriteTimeout。
func TestWithRawResponseWriterClearsWriteTimeout(t *testing.T) {
	handler := WithRawResponseWriter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw, ok := RawResponseWriter(r.Context())
		if !ok {
			t.Fatal("request context 中没有原始 ResponseWriter")
			return
		}
		if err := http.NewResponseController(rw).SetWriteDeadline(time.Time{}); err != nil {
			t.Fatalf("clear write deadline: %v", err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 5; i++ {
			_, _ = fmt.Fprintf(w, "data: %d\n\n", i)
			w.(http.Flusher).Flush()
			time.Sleep(300 * time.Millisecond)
		}
	}))

	server := httptest.NewUnstartedServer(handler)
	server.Config.WriteTimeout = 500 * time.Millisecond
	server.Start()
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := strings.Count(string(body), "data:"); got != 5 {
		t.Fatalf("长连接被提前截断，收到 %d 条: %q", got, string(body))
	}
}
