package iotpcollectorapp

import "testing"

func TestParseIngestEnvelopesSingleAndBatch(t *testing.T) {
	single := []byte(`{"measurement":"m","tags":{"device_id":1},"fields":{"temp":35}}`)
	envs, err := parseIngestEnvelopes(single)
	if err != nil || len(envs) != 1 {
		t.Fatalf("single: envs=%d err=%v", len(envs), err)
	}

	batch := []byte(`[
		{"measurement":"m","tags":{"device_id":1},"fields":{"temp":35}},
		{"measurement":"m","tags":{"device_id":2},"fields":{"temp":36}}
	]`)
	envs, err = parseIngestEnvelopes(batch)
	if err != nil || len(envs) != 2 {
		t.Fatalf("batch: envs=%d err=%v", len(envs), err)
	}
	if envs[1].Tags["device_id"].(float64) != 2 {
		t.Fatalf("batch second device = %v", envs[1].Tags["device_id"])
	}

	if _, err = parseIngestEnvelopes([]byte(`not-json`)); err == nil {
		t.Fatal("invalid payload should fail")
	}
	if _, err = parseIngestEnvelopes(nil); err == nil {
		t.Fatal("empty payload should fail")
	}
}
