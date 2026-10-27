package things

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testConfig(baseURL string) ConfigProvider {
	return func(context.Context) (Config, error) {
		return Config{
			Enabled:       true,
			BaseURL:       baseURL,
			Username:      "svc",
			Password:      "pwd",
			Timeout:       2 * time.Second,
			RefreshBefore: time.Hour,
		}, nil
	}
}

func writeJSON(w http.ResponseWriter, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    code,
		"message": message,
		"data":    data,
	})
}

func TestListProjectsAndTokenHeader(t *testing.T) {
	var loginCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.URL.Path {
		case loginPath:
			if r.Form.Get("username") != "svc" || r.Form.Get("password") != "pwd" {
				t.Errorf("unexpected login form: %v", r.Form)
			}
			atomic.AddInt32(&loginCalls, 1)
			writeJSON(w, 200, "success", map[string]string{"token": "tok-1"})
		case projectListPath:
			if r.Header.Get("Token") != "tok-1" {
				t.Errorf("missing token header: %q", r.Header.Get("Token"))
			}
			writeJSON(w, 200, "success", map[string]interface{}{
				"list": []map[string]interface{}{
					{"id": 101, "name": "Tenant A", "memo": "memo"},
				},
				"totalElements": 1,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(testConfig(srv.URL))
	list, total, err := client.ListProjects(context.Background(), 1, 10, "")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != 101 || list[0].Name != "Tenant A" {
		t.Fatalf("unexpected projects: %+v total=%d", list, total)
	}
	if loginCalls != 1 {
		t.Fatalf("expected 1 login, got %d", loginCalls)
	}
}

func TestRetryAfterUnauthorized(t *testing.T) {
	var loginCalls, listCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case loginPath:
			n := atomic.AddInt32(&loginCalls, 1)
			writeJSON(w, 200, "success", map[string]string{"token": fmt.Sprintf("tok-%d", n)})
		case projectListPath:
			n := atomic.AddInt32(&listCalls, 1)
			if n == 1 {
				// 模拟 things 网关 HTTP 200 + code=401
				writeJSON(w, 401, "UNAUTHORIZED", nil)
				return
			}
			if r.Header.Get("Token") != "tok-2" {
				t.Errorf("expected refreshed token, got %q", r.Header.Get("Token"))
			}
			writeJSON(w, 200, "success", map[string]interface{}{"list": []interface{}{}, "totalElements": 0})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(testConfig(srv.URL))
	if _, _, err := client.ListProjects(context.Background(), 1, 10, ""); err != nil {
		t.Fatalf("ListProjects after 401: %v", err)
	}
	if loginCalls != 2 || listCalls != 2 {
		t.Fatalf("expected 2 logins and 2 list calls, got login=%d list=%d", loginCalls, listCalls)
	}
}

func TestConcurrentRequestsShareOneLogin(t *testing.T) {
	var loginCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case loginPath:
			atomic.AddInt32(&loginCalls, 1)
			time.Sleep(20 * time.Millisecond)
			writeJSON(w, 200, "success", map[string]string{"token": "tok"})
		case projectListPath:
			writeJSON(w, 200, "success", map[string]interface{}{"list": []interface{}{}, "totalElements": 0})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(testConfig(srv.URL))
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := client.ListProjects(context.Background(), 1, 10, "")
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent request: %v", err)
	}
	if loginCalls != 1 {
		t.Fatalf("expected one login, got %d", loginCalls)
	}
}

func TestGatewayListDoesNotExposeSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case loginPath:
			writeJSON(w, 200, "success", map[string]string{"token": "tok"})
		case gatewayListPath:
			writeJSON(w, 200, "success", map[string]interface{}{
				"list": []map[string]interface{}{
					{"id": 1, "projectId": 101, "sn": "SN1", "pk": "SN1", "name": "GW", "model": "M", "status": true, "secret": "should-not-leak"},
				},
				"totalRow": 1,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(testConfig(srv.URL))
	list, err := client.ListGateways(context.Background(), 101)
	if err != nil {
		t.Fatalf("ListGateways: %v", err)
	}
	if len(list) != 1 || list[0].ID != 1 || list[0].SN != "SN1" {
		t.Fatalf("unexpected gateways: %+v", list)
	}
	if got := fmt.Sprintf("%+v", list[0]); strings.Contains(got, "should-not-leak") {
		t.Fatalf("secret leaked into parsed gateway: %s", got)
	}
}

func TestDeleteProjectSendsPassword(t *testing.T) {
	var gotPwd string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case loginPath:
			writeJSON(w, 200, "success", map[string]string{"token": "tok"})
		case projectDeletePath:
			_ = r.ParseForm()
			gotPwd = r.Form.Get("pwd")
			writeJSON(w, 200, "success", nil)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(testConfig(srv.URL))
	if err := client.DeleteProject(context.Background(), 101, "secret-pwd"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if gotPwd != "secret-pwd" {
		t.Fatalf("password not forwarded, got %q", gotPwd)
	}
}
