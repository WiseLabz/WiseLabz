package connector_test

import (
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	_ "github.com/WiseLabz/wiselabz/internal/connector/all"
)

func TestBuiltinCapabilitiesRemainInterfaceBased(t *testing.T) {
	for _, schema := range connector.ListSchemas() {
		if schema.Type == "custom" {
			continue
		}
		instance, err := connector.Get(schema.Type, map[string]any{})
		if err != nil {
			t.Fatalf("Get(%q): %v", schema.Type, err)
		}
		_, restart := instance.(connector.Restarter)
		_, start := instance.(connector.Starter)
		_, stop := instance.(connector.Stopper)
		_, push := instance.(connector.ConfigPusher)
		_, read := instance.(connector.ConfigReader)
		_, refresh := instance.(connector.CredentialRefresher)
		want := connector.CapabilityDescriptor{
			Restart: restart, Start: start, Stop: stop,
			ConfigPush: push, ConfigRead: read, CredentialRefresh: refresh,
		}
		if got := connector.Capabilities(instance); got != want {
			t.Errorf("Capabilities(%q) = %+v, want %+v", schema.Type, got, want)
		}
		for _, verb := range connector.LifecycleVerbs {
			_, wantSupport := connector.LifecycleOp(instance, verb)
			if got := connector.SupportsLifecycleVerb(schema.Type, verb, map[string]any{}); got != wantSupport {
				t.Errorf("SupportsLifecycleVerb(%q, %q) = %t, want %t", schema.Type, verb, got, wantSupport)
			}
		}
	}
}
