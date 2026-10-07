package connector

import "testing"

func TestRelatedSnapshots(t *testing.T) {
	traefikSnapshot := &ServiceSnapshot{ServiceName: "Traefik"}
	want := map[string]*ServiceSnapshot{"traefik-id": traefikSnapshot}
	tests := []struct {
		name   string
		config map[string]any
		want   map[string]*ServiceSnapshot
	}{
		{
			name:   "present",
			config: map[string]any{"_related_snapshots": want},
			want:   want,
		},
		{
			name:   "absent",
			config: map[string]any{},
		},
		{
			name: "nil config",
		},
		{
			name: "JSON-shaped map rejected",
			config: map[string]any{
				"_related_snapshots": map[string]any{
					"traefik-id": map[string]any{"serviceName": "Traefik"},
				},
			},
		},
		{
			name: "wrong snapshot map type rejected",
			config: map[string]any{
				"_related_snapshots": map[string]ServiceSnapshot{
					"traefik-id": {ServiceName: "Traefik"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RelatedSnapshots(tt.config)
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("RelatedSnapshots() = %v, want %v", got, tt.want)
			}
			for id, wantSnapshot := range tt.want {
				if got[id] != wantSnapshot {
					t.Errorf("RelatedSnapshots()[%q] = %p, want %p", id, got[id], wantSnapshot)
				}
			}
		})
	}
}

func TestPreviousSnapshot(t *testing.T) {
	previous := &ServiceSnapshot{ServiceName: "Current connector"}
	tests := []struct {
		name   string
		config map[string]any
		want   *ServiceSnapshot
	}{
		{
			name:   "present",
			config: map[string]any{"_previous_snapshot": previous},
			want:   previous,
		},
		{
			name:   "absent",
			config: map[string]any{},
		},
		{
			name: "nil config",
		},
		{
			name: "JSON-shaped snapshot rejected",
			config: map[string]any{
				"_previous_snapshot": map[string]any{"serviceName": "Current connector"},
			},
		},
		{
			name: "value snapshot rejected",
			config: map[string]any{
				"_previous_snapshot": ServiceSnapshot{ServiceName: "Current connector"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PreviousSnapshot(tt.config); got != tt.want {
				t.Errorf("PreviousSnapshot() = %p, want %p", got, tt.want)
			}
		})
	}
}
