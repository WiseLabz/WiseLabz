package opnsense

import (
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector/connectortest"
)

func TestDiscovery(t *testing.T) { connectortest.RunDiscovery(t, "opnsense") }
