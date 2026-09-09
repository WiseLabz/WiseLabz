package connector

import "testing"

func TestRequestedFields(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   []string
	}{
		{"absent means nil", map[string]any{}, nil},
		{"string slice", map[string]any{"fields": []string{"vms", "storage"}}, []string{"vms", "storage"}},
		{"any slice (typical JSON decode shape)", map[string]any{"fields": []any{"vms", "storage"}}, []string{"vms", "storage"}},
		{"wrong type ignored", map[string]any{"fields": "vms"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RequestedFields(tt.config)
			if len(got) != len(tt.want) {
				t.Fatalf("RequestedFields() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("RequestedFields() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestWantsField(t *testing.T) {
	if !WantsField(nil, "vms") {
		t.Error("WantsField(nil, ...) = false, want true (empty hint means everything)")
	}
	if !WantsField([]string{"vms", "storage"}, "vms") {
		t.Error("WantsField with matching field = false, want true")
	}
	if WantsField([]string{"vms"}, "storage") {
		t.Error("WantsField with non-matching field = true, want false")
	}
}
