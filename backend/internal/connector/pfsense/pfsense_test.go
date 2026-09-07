package pfsense

import (
	"context"
	"strings"
	"testing"
)

func TestStubContract(t *testing.T) {
	c := &Connector{}
	if err := c.Validate(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "stub") {
		t.Fatalf("Validate() error = %v", err)
	}
	snapshot, err := c.Fetch(context.Background(), nil)
	if err != nil || snapshot.Type != typeName || snapshot.Metadata["stub"] != "true" {
		t.Fatalf("Fetch() = %+v, %v", snapshot, err)
	}
}
