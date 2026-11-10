package collectapp

import (
	"context"
	"strings"
	"testing"
)

func TestEngineClientRequiresCollectGateway(t *testing.T) {
	s := &service{}
	_, err := s.engineClient(context.Background())
	if err == nil {
		t.Fatal("engine client without gateway should fail")
	}
	if !strings.Contains(err.Error(), "网关") {
		t.Fatalf("error should tell the user to enter from a gateway, got %v", err)
	}
}
