package iotp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWriteQueryAndTables(t *testing.T) {
	var (
		writePath string
		writeBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/write":
			writePath = r.URL.RequestURI()
			writeBody, _ = io.ReadAll(r.Body)
			_, _ = w.Write([]byte(`{"accepted_rows":1,"rejected_rows":0,"tables":{}}`))
		case "/api/v1/query":
			_, _ = w.Write([]byte(`{"rows":[{"x":1}],"truncated":false,"returned_rows":1,"elapsed_ms":2}`))
		case "/api/v1/meta/tables":
			_, _ = w.Write([]byte(`["p_1"]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := New(srv.URL, "tok", "iotserver", time.Second)
	ctx := context.Background()

	result, err := client.Write(ctx, []WriteRow{{
		Measurement: "p_1",
		Tags:        map[string]any{"device_id": 7},
		Fields:      map[string]any{"temperature": 23.5},
		Timestamp:   1700000000000,
	}})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if result.AcceptedRows != 1 {
		t.Fatalf("accepted = %d, want 1", result.AcceptedRows)
	}
	if writePath != "/write?db=iotserver" {
		t.Errorf("写路径 = %q, want /write?db=iotserver", writePath)
	}
	body := string(writeBody)
	for _, want := range []string{`"measurement":"p_1"`, `"device_id":7`, `"temperature":23.5`, `"timestamp":1700000000000`} {
		if !strings.Contains(body, want) {
			t.Errorf("写入报文缺少 %s：%s", want, body)
		}
	}

	query, err := client.Query(ctx, "SELECT 1", "")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(query.Rows) != 1 || query.Rows[0]["x"] != float64(1) {
		t.Fatalf("查询结果 = %+v", query)
	}

	tables, err := client.Tables(ctx)
	if err != nil {
		t.Fatalf("Tables: %v", err)
	}
	if len(tables) != 1 || tables[0] != "p_1" {
		t.Fatalf("tables = %+v", tables)
	}
}

func TestWriteResultReasons(t *testing.T) {
	result := WriteResult{
		AcceptedRows: 1,
		RejectedRows: 2,
		Tables: map[string]WriteTableResult{
			"iotserver.p_1": {Accepted: 1, Rejected: 2, Reasons: map[string]int{"unknown_field": 2}},
		},
	}
	if result.Rejected() != 2 || result.Reason("unknown_field") != 2 || result.Reason("unknown_device") != 0 {
		t.Fatalf("结果统计错误: %+v", result)
	}
}
