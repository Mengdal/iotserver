package gatewayapp

import "testing"

func TestNormalizeEngineConfig(t *testing.T) {
	base, timeout, err := normalizeEngineConfig(" http://192.168.0.74:8081/ ", 0)
	if err != nil {
		t.Fatalf("normalize valid url: %v", err)
	}
	if base != "http://192.168.0.74:8081" {
		t.Fatalf("unexpected base url: %q", base)
	}
	if timeout != 10 {
		t.Fatalf("default timeout should be 10, got %d", timeout)
	}

	if _, _, err := normalizeEngineConfig("", 10); err == nil {
		t.Fatal("empty engine url should fail")
	}
	if _, _, err := normalizeEngineConfig("192.168.0.74:8081", 10); err == nil {
		t.Fatal("url without scheme should fail")
	}

	_, timeout, err = normalizeEngineConfig("https://engine.example.com", 999)
	if err != nil {
		t.Fatalf("normalize https url: %v", err)
	}
	if timeout != 300 {
		t.Fatalf("timeout should be capped at 300, got %d", timeout)
	}
}
