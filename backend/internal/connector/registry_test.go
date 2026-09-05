package connector

import "testing"

func TestRegisterStubRoundTrips(t *testing.T) {
	Register(TypeSchema{Type: "registry_test_stub", Category: "test", Name: "Stub", Stub: true},
		func(_ map[string]any) (Connector, error) { return nil, nil })

	got, err := GetTypeSchema("registry_test_stub")
	if err != nil {
		t.Fatalf("GetTypeSchema: %v", err)
	}
	if !got.Stub {
		t.Errorf("Stub = false, want true")
	}

	found := false
	for _, s := range ListSchemas() {
		if s.Type == "registry_test_stub" {
			found = true
			if !s.Stub {
				t.Errorf("ListSchemas: Stub = false, want true")
			}
		}
	}
	if !found {
		t.Fatal("registry_test_stub not found in ListSchemas()")
	}
}

func TestRegisterDefaultsToNonStub(t *testing.T) {
	Register(TypeSchema{Type: "registry_test_real", Category: "test", Name: "Real"},
		func(_ map[string]any) (Connector, error) { return nil, nil })

	got, err := GetTypeSchema("registry_test_real")
	if err != nil {
		t.Fatalf("GetTypeSchema: %v", err)
	}
	if got.Stub {
		t.Errorf("Stub = true, want false")
	}
}
