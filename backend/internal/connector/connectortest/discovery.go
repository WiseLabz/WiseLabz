package connectortest

import (
	"net/http"
	"sort"
	"sync"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// DiscoveryFixture is a response to one discovery probe request, written from
// what the product is documented or known to return, not captured from a
// live instance. Match says whether it identifies the product under test: a
// false fixture is another product or a generic server answering that port.
type DiscoveryFixture struct {
	Name     string
	Port     int
	Path     string // the probe request the response answers
	Response connector.DiscoveryResponse
	Match    bool
}

var (
	fixturesMu sync.Mutex
	fixtures   = map[string][]DiscoveryFixture{}
)

// RegisterDiscoveryFixtures adds fixtures for a connector type. Call it from
// an init in a discovery_fixtures_<type>.go file in this package.
func RegisterDiscoveryFixtures(typ string, fx ...DiscoveryFixture) {
	fixturesMu.Lock()
	defer fixturesMu.Unlock()
	fixtures[typ] = append(fixtures[typ], fx...)
}

// DiscoveryFixtures returns every registered fixture by connector type.
func DiscoveryFixtures() map[string][]DiscoveryFixture {
	fixturesMu.Lock()
	defer fixturesMu.Unlock()
	out := make(map[string][]DiscoveryFixture, len(fixtures))
	for k, v := range fixtures {
		out[k] = append([]DiscoveryFixture(nil), v...)
	}
	return out
}

// Response builds a DiscoveryResponse for a fixture. headers are alternating
// name, value pairs.
func Response(status int, body string, headers ...string) connector.DiscoveryResponse {
	h := http.Header{}
	for i := 0; i+1 < len(headers); i += 2 {
		h.Add(headers[i], headers[i+1])
	}
	return connector.DiscoveryResponse{Status: status, Header: h, Body: []byte(body)}
}

// GenericWebResponses are pages a plain web server returns for any probe; no
// probe of any type may match them.
func GenericWebResponses() []connector.DiscoveryResponse {
	return []connector.DiscoveryResponse{
		Response(200, "<!DOCTYPE html><html><head><title>Welcome to nginx!</title></head><body><h1>Welcome to nginx!</h1></body></html>", "Server", "nginx/1.25.3", "Content-Type", "text/html"),
		Response(200, "<html><body><h1>It works!</h1></body></html>", "Server", "Apache/2.4.58 (Debian)", "Content-Type", "text/html"),
		Response(200, "<html><head><title>Index of /</title></head><body><h1>Index of /</h1></body></html>", "Server", "lighttpd/1.4.71"),
		Response(404, "404 page not found\n", "Content-Type", "text/plain; charset=utf-8"),
		Response(404, `{"error":"not found"}`, "Content-Type", "application/json"),
		Response(401, ""),
		Response(403, "Forbidden"),
		Response(302, "", "Location", "/login"),
		Response(301, "", "Location", "https://10.0.0.1/"),
		Response(200, ""),
		Response(200, `{}`, "Content-Type", "application/json"),
		Response(200, `[]`, "Content-Type", "application/json"),
		Response(200, `null`, "Content-Type", "application/json"),
		Response(200, `{"status":"ok"}`, "Content-Type", "application/json"),
		Response(200, `<html><head><title>Login</title></head><body><form><input name="username"><input name="password" type="password"></form></body></html>`),
		Response(200, `{"name":"My App","version":"1.0.0"}`, "Content-Type", "application/json"),
	}
}

// RunDiscovery checks a connector type's discovery hint against its fixtures
// and against GenericWebResponses: every fixture must be judged as its Match
// field says by the type's probes on that fixture's port, every probe must be
// exercised by a fixture, and no probe may match a generic web page.
func RunDiscovery(t *testing.T, typ string) {
	t.Helper()
	var hint *connector.TypeDiscovery
	for _, d := range connector.DiscoveryHints() {
		if d.Type == typ {
			d := d
			hint = &d
		}
	}
	if hint == nil {
		t.Fatalf("connector type %q declares no discovery hint", typ)
	}
	if hint.URLTemplate == "" {
		t.Errorf("%s: empty URLTemplate", typ)
	}
	if len(hint.Probes) == 0 {
		t.Fatalf("%s: no probes", typ)
	}
	fx := DiscoveryFixtures()[typ]
	if len(fx) == 0 {
		t.Fatalf("%s: no discovery fixtures registered", typ)
	}

	exercised := make([]bool, len(hint.Probes))
	for i, p := range hint.Probes {
		if p.Port <= 0 || p.Port > 65535 || p.Match == nil || p.Path == "" || p.Path[0] != '/' || (p.Scheme != "http" && p.Scheme != "https") {
			t.Errorf("%s: malformed probe %d: %+v", typ, i, p)
		}
	}
	for _, f := range fx {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			matched, seen := false, false
			for i, p := range hint.Probes {
				if p.Port != f.Port || p.Path != f.Path {
					continue
				}
				seen = true
				exercised[i] = true
				if p.Match(f.Response) {
					matched = true
				}
			}
			if !seen {
				t.Fatalf("no probe on port %d path %q", f.Port, f.Path)
			}
			if matched != f.Match {
				t.Errorf("probe match = %v, want %v", matched, f.Match)
			}
		})
	}
	for i, ok := range exercised {
		if !ok {
			t.Errorf("%s: probe %d (%s :%d %s) has no fixture", typ, i, hint.Probes[i].Scheme, hint.Probes[i].Port, hint.Probes[i].Path)
		}
	}
	for i, g := range GenericWebResponses() {
		for _, p := range hint.Probes {
			if p.Match(g) {
				t.Errorf("%s: probe :%d %s matches generic web response %d (status %d)", typ, p.Port, p.Path, i, g.Status)
			}
		}
	}
}

// RunDiscoveryCross checks every positive fixture against the probes of every
// other type that listens on the same port: a probe must not mistake another
// product's answer for its own. It returns the types it covered.
func RunDiscoveryCross(t *testing.T) []string {
	t.Helper()
	hints := connector.DiscoveryHints()
	all := DiscoveryFixtures()
	var types []string
	for typ := range all {
		types = append(types, typ)
	}
	sort.Strings(types)
	for _, a := range types {
		for _, f := range all[a] {
			if !f.Match {
				continue
			}
			for _, b := range hints {
				if b.Type == a {
					continue
				}
				for _, p := range b.Probes {
					if p.Port == f.Port && p.Match(f.Response) {
						t.Errorf("%s fixture %q is matched by the %s probe on port %d %s", a, f.Name, b.Type, p.Port, p.Path)
					}
				}
			}
		}
	}
	return types
}
