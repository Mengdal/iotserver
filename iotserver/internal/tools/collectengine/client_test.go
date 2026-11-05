package collectengine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/channels" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    200,
			"message": "ok",
			"data": []map[string]any{
				{"id": 1, "channelCode": "C1", "protocol": "ModbusTCPClient", "comInterval": 1000},
			},
		})
	}))
	defer server.Close()

	channels, err := New(server.URL, 0).ListChannels()
	if err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if len(channels) != 1 || channels[0].ChannelCode != "C1" || channels[0].Id != 1 {
		t.Fatalf("unexpected channels: %+v", channels)
	}
}

func TestCreateChannelUsesFormData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/channels" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v", err)
		}
		if got := r.FormValue("channelCode"); got != "C1" {
			t.Errorf("channelCode = %q, want C1", got)
		}
		if got := r.FormValue("comInterval"); got != "1000" {
			t.Errorf("comInterval = %q, want 1000", got)
		}
		if got := r.FormValue("disable"); got != "false" {
			t.Errorf("disable = %q, want false", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"data": map[string]any{"id": 7, "channelCode": "C1"},
		})
	}))
	defer server.Close()

	created, err := New(server.URL, 0).CreateChannel(Channel{
		ChannelCode: "C1",
		Protocol:    "ModbusTCPClient",
		ComInterval: 1000,
		Advance:     "{}",
	})
	if err != nil {
		t.Fatalf("CreateChannel() error = %v", err)
	}
	if created.Id != 7 {
		t.Fatalf("created.Id = %d, want 7", created.Id)
	}
}

func TestBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    400,
			"message": "channelCode 不能为空",
		})
	}))
	defer server.Close()

	_, err := New(server.URL, 0).ListChannels()
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != 400 {
		t.Fatalf("err = %#v, want *APIError with code 400", err)
	}
}

func TestDeleteTag(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "message": "删除点位成功"})
	}))
	defer server.Close()

	if err := New(server.URL, 0).DeleteTag(9); err != nil {
		t.Fatalf("DeleteTag() error = %v", err)
	}
	if gotPath != "/api/v1/tags/9" {
		t.Fatalf("path = %q, want /api/v1/tags/9", gotPath)
	}
}

func TestListDevicesFiltersChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/devices" || r.URL.Query().Get("channelId") != "7" {
			t.Fatalf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"data": []map[string]any{{"id": 3, "channelId": 7, "iotpId": 1001}},
		})
	}))
	defer server.Close()

	devices, err := New(server.URL, 0).ListDevices(7)
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}
	if len(devices) != 1 || devices[0].Id != 3 || devices[0].ChannelId != 7 || devices[0].IotpId != 1001 {
		t.Fatalf("unexpected devices: %+v", devices)
	}
}

func TestDeleteDeviceByID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/devices/3" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "message": "删除成功"})
	}))
	defer server.Close()

	if err := New(server.URL, 0).DeleteDevice(3); err != nil {
		t.Fatalf("DeleteDevice() error = %v", err)
	}
}

func TestRealtimeTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/tags/realtime" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("deviceId"); got != "9" {
			t.Fatalf("deviceId = %q, want 9", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"data": map[string]any{
				"items": []map[string]any{
					{"id": "17156557.oil_temp", "status": 192, "value": "2.000000", "timestamp": 1793839480},
					{"id": "17156557.oil_pressure", "status": 0, "value": "0", "timestamp": 0},
					{"id": "17156557.run_status", "status": "Uncertain", "value": "0", "timestamp": 0},
				},
				"total":    3,
				"page":     1,
				"pageSize": 1000,
			},
		})
	}))
	defer server.Close()

	page, err := New(server.URL, 0).RealtimeTags(9)
	if err != nil {
		t.Fatalf("RealtimeTags() error = %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("want 3 items, got %d", len(page.Items))
	}
	if page.Items[0].TagCode() != "oil_temp" || page.Items[0].StatusText() != "Good" {
		t.Fatalf("unexpected first record: %+v", page.Items[0])
	}
	if page.Items[1].StatusText() != "Error" {
		t.Fatalf("status 0 should be Error: %+v", page.Items[1])
	}
	if page.Items[2].StatusText() != "Uncertain" {
		t.Fatalf("unexpected string status: %+v", page.Items[2])
	}
}

func TestTagRealtimeStatusText(t *testing.T) {
	cases := []struct {
		status any
		want   string
	}{
		{float64(192), "Good"},
		{float64(0), "Error"},
		{"Good", "Good"},
		{"Error", "Error"},
		{"Uncertain", "Uncertain"},
		{nil, "Uncertain"},
	}
	for _, item := range cases {
		record := TagRealtimeRecord{Status: item.status}
		if got := record.StatusText(); got != item.want {
			t.Errorf("StatusText(%v) = %q, want %q", item.status, got, item.want)
		}
	}
}

func TestWriteTagUsesFormData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/tags/write" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v", err)
		}
		if got := r.FormValue("deviceId"); got != "9" {
			t.Errorf("deviceId = %q, want 9", got)
		}
		if got := r.FormValue("tagCode"); got != "oil_temp" {
			t.Errorf("tagCode = %q, want oil_temp", got)
		}
		if got := r.FormValue("value"); got != "12.3" {
			t.Errorf("value = %q, want 12.3", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "message": "ok"})
	}))
	defer server.Close()

	if err := New(server.URL, 0).WriteTag(9, "oil_temp", "12.3"); err != nil {
		t.Fatalf("WriteTag() error = %v", err)
	}
}
