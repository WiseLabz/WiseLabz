package auth

import (
	"reflect"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/config"
)

func TestWebAuthnRPConfig(t *testing.T) {
	cfg := &config.Config{Server: config.Server{Origin: "http://localhost:5173, https://lab.example.com"}}
	id, name, origins, ok := WebAuthnRPConfig(cfg)
	if !ok || id != "localhost" || name != "WiseLabz" || !reflect.DeepEqual(origins, []string{"http://localhost:5173", "https://lab.example.com"}) {
		t.Fatalf("derived RP config: id=%q name=%q origins=%v ok=%v", id, name, origins, ok)
	}
	cfg.Auth.WebAuthn.RPID = "example.com"
	cfg.Auth.WebAuthn.RPDisplayName = "Lab"
	id, name, _, ok = WebAuthnRPConfig(cfg)
	if !ok || id != "example.com" || name != "Lab" {
		t.Fatalf("overridden RP config: id=%q name=%q ok=%v", id, name, ok)
	}
	cfg.Server.Origin = ""
	if _, _, _, ok := WebAuthnRPConfig(cfg); ok {
		t.Fatal("WebAuthn should be disabled without a usable origin")
	}
}
