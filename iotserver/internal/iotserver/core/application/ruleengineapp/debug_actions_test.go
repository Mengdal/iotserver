package ruleengineapp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/logger"
)

func TestExecuteDebugHTTPAction(t *testing.T) {
	var (
		gotMethod string
		gotCT     string
		gotHeader string
		gotBody   string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		gotHeader = r.Header.Get("X-Debug-Test")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	action := debugAction{
		resourceID:   "r1",
		name:         "webhook",
		resourceType: "HTTP推送",
		props: dtos.Actions{Rest: map[string]interface{}{
			"url":      server.URL,
			"method":   "POST",
			"bodyType": "json",
			"timeout":  5000,
			"headers":  map[string]interface{}{"X-Debug-Test": "yes"},
		}},
	}
	result := executeDebugAction(context.Background(), action, []byte(`{"temperature":32.5}`))
	if !result.Ok {
		t.Fatalf("result = %#v", result)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", result.StatusCode)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q", gotMethod)
	}
	if gotCT != "application/json" {
		t.Fatalf("content type = %q", gotCT)
	}
	if gotHeader != "yes" {
		t.Fatalf("header = %q", gotHeader)
	}
	if gotBody != `{"temperature":32.5}` {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestExecuteDebugHTTPActionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream down"))
	}))
	defer server.Close()

	action := debugAction{
		resourceID:   "r2",
		name:         "webhook",
		resourceType: "HTTP推送",
		props: dtos.Actions{Rest: map[string]interface{}{
			"url":     server.URL,
			"method":  "POST",
			"timeout": 5000,
		}},
	}
	result := executeDebugAction(context.Background(), action, []byte(`{"a":1}`))
	if result.Ok {
		t.Fatalf("expected failure, got %#v", result)
	}
	if result.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", result.StatusCode)
	}
	if !strings.Contains(result.Error, "502") || !strings.Contains(result.Error, "upstream down") {
		t.Fatalf("error = %q", result.Error)
	}
}

func TestRuleEngineDebugDeliverSkipsEngineErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	app := &ruleEngineApp{
		lc: logger.MockLogger{},
		debugSessions: map[string]debugSession{
			"test_1_1": {
				actions: []debugAction{{
					resourceID:   "r1",
					name:         "webhook",
					resourceType: "HTTP推送",
					props: dtos.Actions{Rest: map[string]interface{}{
						"url": server.URL, "method": "POST", "timeout": 5000,
					}},
				}},
			},
		},
	}
	if results := app.RuleEngineDebugDeliver(context.Background(), "test_1_1", "fail to calculate props"); results != nil {
		t.Fatalf("engine error should not be delivered: %#v", results)
	}
	if results := app.RuleEngineDebugDeliver(context.Background(), "test_1_1", `{"ok":true}`); len(results) != 1 || !results[0].Ok {
		t.Fatalf("valid payload should be delivered: %#v", results)
	}
	if results := app.RuleEngineDebugDeliver(context.Background(), "test_missing_1", `{"ok":true}`); results != nil {
		t.Fatalf("missing session should be ignored: %#v", results)
	}
}

func TestRuleEngineDebugDeliverAllowsTemplatedText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	app := &ruleEngineApp{
		lc: logger.MockLogger{},
		debugSessions: map[string]debugSession{
			"test_1_1": {
				hasDataTemplate: true,
				actions: []debugAction{{
					resourceID:   "r1",
					name:         "webhook",
					resourceType: "HTTP推送",
					props: dtos.Actions{Rest: map[string]interface{}{
						"url": server.URL, "method": "POST", "bodyType": "text", "timeout": 5000,
					}},
				}},
			},
		},
	}
	results := app.RuleEngineDebugDeliver(context.Background(), "test_1_1", "plain text body")
	if len(results) != 1 || !results[0].Ok {
		t.Fatalf("templated text should be delivered: %#v", results)
	}
}
