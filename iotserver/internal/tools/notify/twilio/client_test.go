package twilio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in   string
		cc   string
		want string
	}{
		{"+966551234567", "966", "+966551234567"},
		{"00966551234567", "", "+966551234567"},
		{"0551234567", "966", "+966551234567"},
		{"551234567", "966", "+966551234567"},
		{"966551234567", "966", "+966551234567"},
		{"", "966", ""},
	}
	for _, tc := range cases {
		if got := NormalizePhone(tc.in, tc.cc); got != tc.want {
			t.Errorf("NormalizePhone(%q, %q) = %q, want %q", tc.in, tc.cc, got, tc.want)
		}
	}
}

func TestSendSMS(t *testing.T) {
	var gotTo, gotFrom, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		gotTo = r.FormValue("To")
		gotFrom = r.FormValue("From")
		gotBody = r.FormValue("Body")
		user, pass, ok := r.BasicAuth()
		if !ok || user != "AC123" || pass != "token" {
			t.Errorf("unexpected basic auth: %q %q %v", user, pass, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sid":"SM123","status":"queued"}`))
	}))
	defer server.Close()

	client := New(Config{
		AccountSID:         "AC123",
		AuthToken:          "token",
		SMSFrom:            "+14155550100",
		DefaultCountryCode: "966",
		BaseURL:            server.URL,
	})
	if err := client.SendSMS(context.Background(), "0551234567", "temperature high"); err != nil {
		t.Fatalf("send sms: %v", err)
	}
	if gotTo != "+966551234567" {
		t.Errorf("To = %q", gotTo)
	}
	if gotFrom != "+14155550100" {
		t.Errorf("From = %q", gotFrom)
	}
	if gotBody != "temperature high" {
		t.Errorf("Body = %q", gotBody)
	}
}

func TestSendWhatsAppTemplate(t *testing.T) {
	var gotTo, gotFrom, contentSID string
	var variables map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotTo = r.FormValue("To")
		gotFrom = r.FormValue("From")
		contentSID = r.FormValue("ContentSid")
		_ = json.Unmarshal([]byte(r.FormValue("ContentVariables")), &variables)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sid":"SM456","status":"queued"}`))
	}))
	defer server.Close()

	client := New(Config{
		AccountSID:         "AC123",
		AuthToken:          "token",
		WhatsAppFrom:       "+14155550100",
		DefaultCountryCode: "966",
		BaseURL:            server.URL,
	})
	vars := map[string]string{"1": "Cold room A", "2": "High temperature"}
	if err := client.SendWhatsApp(context.Background(), "0551234567", "fallback", "HX123", vars); err != nil {
		t.Fatalf("send whatsapp: %v", err)
	}
	if gotTo != "whatsapp:+966551234567" {
		t.Errorf("To = %q", gotTo)
	}
	if gotFrom != "whatsapp:+14155550100" {
		t.Errorf("From = %q", gotFrom)
	}
	if contentSID != "HX123" {
		t.Errorf("ContentSid = %q", contentSID)
	}
	if variables["1"] != "Cold room A" || variables["2"] != "High temperature" {
		t.Errorf("ContentVariables = %#v", variables)
	}
}

func TestTwilioError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":21211,"message":"Invalid To number"}`))
	}))
	defer server.Close()

	client := New(Config{AccountSID: "AC123", AuthToken: "token", SMSFrom: "+1", BaseURL: server.URL})
	err := client.SendSMS(context.Background(), "+966500000000", "test")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSendEmail(t *testing.T) {
	var (
		gotPath string
		payload struct {
			From struct {
				Address string `json:"address"`
				Name    string `json:"name"`
			} `json:"from"`
			To []struct {
				Address string `json:"address"`
			} `json:"to"`
			Content struct {
				Subject string `json:"subject"`
				Text    string `json:"text"`
				HTML    string `json:"html"`
			} `json:"content"`
		}
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "AC123" || pass != "token" {
			t.Errorf("unexpected basic auth: %q %q %v", user, pass, ok)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"operationId":"op-1"}`))
	}))
	defer server.Close()

	client := New(Config{
		AccountSID:   "AC123",
		AuthToken:    "token",
		EmailFrom:    "Alerts <alerts@example.com>",
		EmailBaseURL: server.URL,
	})
	if err := client.SendEmail(context.Background(), "ops@example.com, oncall@example.com", "Device alarm", "line1\nline2"); err != nil {
		t.Fatalf("send email: %v", err)
	}
	if gotPath != "/v1/Emails" {
		t.Errorf("path = %q", gotPath)
	}
	if payload.From.Address != "alerts@example.com" || payload.From.Name != "iotserver" {
		t.Errorf("from = %#v", payload.From)
	}
	if len(payload.To) != 2 || payload.To[0].Address != "ops@example.com" || payload.To[1].Address != "oncall@example.com" {
		t.Errorf("to = %#v", payload.To)
	}
	if payload.Content.Subject != "Device alarm" || payload.Content.Text != "line1\nline2" {
		t.Errorf("content = %#v", payload.Content)
	}
	if payload.Content.HTML != "line1<br>\nline2" {
		t.Errorf("html = %q", payload.Content.HTML)
	}
}
