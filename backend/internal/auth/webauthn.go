package auth

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/WiseLabz/wiselabz/internal/config"
)

// WebAuthnRPConfig derives the relying-party ID and allowed origins from
// Server.Origin — the same comma-separated list api/auth's oidcRedirectURL
// parses — so WebAuthn (#279 part 2) needs no separate origin config in the
// common case. RP ID defaults to the first valid origin's hostname (without
// port); auth.webauthn.rp_id/rp_display_name override the derived values.
// ok is false when no usable origin is configured, in which case callers
// must treat WebAuthn as disabled instance-wide.
func WebAuthnRPConfig(cfg *config.Config) (rpID, rpDisplayName string, origins []string, ok bool) {
	for _, o := range strings.Split(cfg.Server.Origin, ",") {
		u, err := url.Parse(strings.TrimSpace(o))
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		origins = append(origins, u.Scheme+"://"+u.Host)
		if rpID == "" {
			rpID = u.Hostname()
		}
	}
	if cfg.Auth.WebAuthn.RPID != "" {
		rpID = cfg.Auth.WebAuthn.RPID
	}
	rpDisplayName = "WiseLabz"
	if cfg.Auth.WebAuthn.RPDisplayName != "" {
		rpDisplayName = cfg.Auth.WebAuthn.RPDisplayName
	}
	if len(origins) == 0 || rpID == "" {
		return "", "", nil, false
	}
	return rpID, rpDisplayName, origins, true
}

// NewWebAuthnService builds the relying-party instance used for every
// WebAuthn ceremony from cfg. ok is false when no usable origin is
// configured (WebAuthnRPConfig), in which case WebAuthn is disabled:
// GET /auth/elevate/methods and the login "methods" list never mention it,
// and the registration endpoints return 409 webauthn_unavailable. Callers
// build this once at startup and log a single warning when ok is false,
// rather than re-deriving it (and re-warning) per request.
func NewWebAuthnService(cfg *config.Config) (svc *webauthn.WebAuthn, ok bool, err error) {
	rpID, rpDisplayName, origins, ok := WebAuthnRPConfig(cfg)
	if !ok {
		return nil, false, nil
	}
	svc, err = webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpDisplayName,
		RPOrigins:     origins,
	})
	if err != nil {
		return nil, false, fmt.Errorf("new webauthn service: %w", err)
	}
	return svc, true, nil
}
