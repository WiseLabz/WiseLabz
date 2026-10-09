package custom

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestParseRecipeRejectsPlaceholdersInPathQueryString(t *testing.T) {
	base := validActionRecipeForFormat()
	const rescanPath = "          path: /api/nodes/{attr.node}/containers/{external_id}/rescan\n"
	for _, tt := range []struct{ name, path, want string }{
		{"external id", "/items/act?id={external_id}", "endpoints[0].entity.actions.rescan.path"},
		{"attribute", "/items/{external_id}/act?node={attr.node}", "endpoints[0].entity.actions.rescan.path"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := strings.Replace(base, rescanPath, "          path: \""+tt.path+"\"\n", 1)
			_, err := ParseRecipe(raw)
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), "query string of path") {
				t.Fatalf("ParseRecipe() error = %v, want an issue at %s", err, tt.want)
			}
		})
	}
	static := strings.Replace(base, rescanPath, "          path: \"/items/{external_id}/act?mode=fast\"\n", 1)
	if _, err := ParseRecipe(static); err != nil {
		t.Fatalf("static query string in path rejected: %v", err)
	}
}

func mustParseQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Query()
}

func hardeningEntityRecipe(actionPath string) string {
	return `version: 1
category: media
auth: {mode: none}
endpoints:
  - name: items
    path: /api/items
    method: GET
    items: items
    entity:
      kind: item
      name: name
      external_id: id
      attributes:
        size: {path: size}
      actions:
        act:
          method: POST
          path: "` + actionPath + `"
          query: {id: "{external_id}", size: "{attr.size}"}
          body: {label: "{attr.size}"}
`
}

func TestResolveActionKeepsStaticPathQueryAndRejectsPlaceholders(t *testing.T) {
	snapshot := &connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{{Kind: "item", ExternalID: "abc", Attributes: map[string]any{"size": float64(3)}}}}
	conn := &Connector{}

	config := map[string]any{"url": "https://svc.example.com", "recipe": hardeningEntityRecipe("/items/{external_id}/act?mode=fast")}
	resolved, err := conn.ResolveAction(config, "act", "abc", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	parsed := resolved.Request.URL
	if !strings.HasPrefix(parsed, "https://svc.example.com/items/abc/act?") {
		t.Fatalf("resolved URL = %q", parsed)
	}
	query := mustParseQuery(t, parsed)
	if len(query) != 3 || query.Get("mode") != "fast" || query.Get("id") != "abc" || query.Get("size") != "3" {
		t.Fatalf("resolved query = %v, want mode, id and size only", query)
	}

	for _, path := range []string{"/items/act?id={external_id}", "/items/{external_id}?x={attr.size}"} {
		_, err := resolveRecipeAction(RecipeAction{Method: "POST", Path: path}, &snapshot.Entities[0])
		if err == nil || !strings.Contains(err.Error(), "query string of path") {
			t.Errorf("resolveRecipeAction(%q) error = %v, want a query-string placeholder error", path, err)
		}
	}
	unescaped, err := resolveRecipeAction(RecipeAction{Method: "POST", Path: "/items/{external_id}?note={{x}}"}, &snapshot.Entities[0])
	if err != nil || unescaped.Path != "/items/abc?note={x}" {
		t.Errorf("resolveRecipeAction(escaped braces) = %q, %v", unescaped.Path, err)
	}
}

func TestSendActionKeepsEntityValueInsideOneQueryParameter(t *testing.T) {
	const external = "x&force=true"
	var calls atomic.Int32
	var gotQuery map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		gotQuery = r.URL.Query()
		_, _ = io.WriteString(w, "OK")
	}))
	defer server.Close()
	recipe := strings.Replace(hardeningEntityRecipe("/api/act"), `, size: "{attr.size}"`, "", 1)
	config := map[string]any{"url": server.URL, "recipe": recipe}
	snapshot := &connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{{Kind: "item", ExternalID: external, Attributes: map[string]any{"size": 1.0}}}}
	conn := &Connector{client: server.Client()}
	action, err := conn.ResolveAction(config, "act", external, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.SendAction(context.Background(), config, action); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || len(gotQuery) != 1 || len(gotQuery["id"]) != 1 || gotQuery["id"][0] != external {
		t.Fatalf("upstream calls=%d query=%v, want exactly one id=%q", calls.Load(), gotQuery, external)
	}
}

func TestActionNumericAttributesUsePlainDecimalNotation(t *testing.T) {
	var snapshot connector.ServiceSnapshot
	if err := json.Unmarshal([]byte(`{"entities":[{"kind":"item","externalId":"abc","attributes":{"size":1234567,"ratio":1.5}}]}`), &snapshot); err != nil {
		t.Fatal(err)
	}
	if _, ok := snapshot.Entities[0].Attributes["size"].(float64); !ok {
		t.Fatalf("decoded attribute is %T, want float64", snapshot.Entities[0].Attributes["size"])
	}
	for _, tt := range []struct {
		name  string
		attrs map[string]any
		want  string
	}{
		{"float64 millions", map[string]any{"n": float64(1234567)}, "1234567"},
		{"float64 fraction", map[string]any{"n": 1.5}, "1.5"},
		{"float32", map[string]any{"n": float32(2500000)}, "2500000"},
		{"json round trip", map[string]any{"n": snapshot.Entities[0].Attributes["size"]}, "1234567"},
		{"json round trip fraction", map[string]any{"n": snapshot.Entities[0].Attributes["ratio"]}, "1.5"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entity := &connector.SnapshotEntity{ExternalID: "abc", Attributes: tt.attrs}
			resolved, err := resolveRecipeAction(RecipeAction{
				Method: "POST",
				Path:   "/items/{attr.n}/act",
				Query:  map[string]string{"n": "{attr.n}"},
				Body:   map[string]any{"text": "n={attr.n}"},
			}, entity)
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Path != "/items/"+tt.want+"/act" || resolved.Query["n"] != tt.want || resolved.Body.(map[string]any)["text"] != "n="+tt.want {
				t.Errorf("resolved = %+v, want %q everywhere", resolved, tt.want)
			}
		})
	}
}

func TestSendActionReportsWrittenWhenARequestFailsPartWay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack() error = %v", err)
			return
		}
		_, _ = buffered.Read(make([]byte, 1024))
		_ = conn.Close()
	}))
	defer server.Close()
	config := map[string]any{"url": server.URL, "recipe": serviceActionRecipe(http.MethodPost)}
	conn := &Connector{client: server.Client()}
	action, err := conn.ResolveAction(config, "operation", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	// A recipe is limited to 64 KiB, so the large body is set on the resolved request.
	action.Request.Body = strings.Repeat("a", 1<<20)
	result, err := conn.SendAction(context.Background(), config, action)
	if err == nil || !result.Written || result.Status != 0 {
		t.Errorf("partial write result/error = %+v / %v, want an error with Written and no status", result, err)
	}
}

func TestSendActionRefusedConnectionWasNotWritten(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	config := map[string]any{"url": "http://" + address, "recipe": serviceActionRecipe(http.MethodPost)}
	conn := &Connector{client: http.DefaultClient}
	action, err := conn.ResolveAction(config, "operation", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := conn.SendAction(context.Background(), config, action)
	if err == nil || result.Written || result.Status != 0 {
		t.Errorf("refused result/error = %+v / %v, want an error with Written false", result, err)
	}
}

func TestActionTextExcerptJudgesOnlyTheFirstBytes(t *testing.T) {
	late := []byte(strings.Repeat("a", 550) + "\xff" + strings.Repeat("b", 49))
	if got := actionTextExcerpt("text/plain", late); got != strings.Repeat("a", 512) {
		t.Errorf("invalid byte after the prefix: excerpt length %d, want the first 512 bytes", len(got))
	}

	for offset := 510; offset <= 512; offset++ {
		straddling := []byte(strings.Repeat("a", offset) + "€" + strings.Repeat("b", 100))
		got := actionTextExcerpt("text/plain", straddling)
		if !utf8.ValidString(got) || len(got) > actionExcerptBytes || got != strings.Repeat("a", offset) {
			t.Errorf("rune at offset %d: excerpt length %d valid=%v, want %d a's", offset, len(got), utf8.ValidString(got), offset)
		}
	}

	capture := []byte(strings.Repeat("a", 510) + "€")[:actionExcerptBytes]
	if got := actionTextExcerpt("text/plain", capture); got != strings.Repeat("a", 510) {
		t.Errorf("cut capture excerpt length = %d, want 510", len(got))
	}

	binary := append([]byte{0xff, 0xfe, 0x00, 0x01}, []byte(strings.Repeat("a", 600))...)
	if got := actionTextExcerpt("text/plain", binary); got != "" {
		t.Errorf("binary prefix excerpt = %q, want empty", got)
	}
	if got := actionTextExcerpt("text/plain", []byte("ok\xff")); got != "" {
		t.Errorf("short invalid body excerpt = %q, want empty", got)
	}
}

func TestActionTextExcerptDropsFormatCharacters(t *testing.T) {
	got := actionTextExcerpt("text/plain", []byte("before\u202eevil\u2066x\u200bafter"))
	if got != "beforeevilxafter" {
		t.Errorf("excerpt = %q, want the text without bidirectional or zero-width characters", got)
	}
}

func TestParseRecipeRejectsUnusableAttributeNamesInPlaceholders(t *testing.T) {
	recipe := func(attribute, action string) string {
		return `version: 1
category: media
auth: {mode: none}
endpoints:
  - name: items
    path: /api/items
    method: GET
    items: items
    entity:
      kind: item
      name: name
      external_id: id
      attributes:
        "` + attribute + `": {path: size}
      actions:
        act:
          method: POST
` + action
	}
	for _, tt := range []struct{ name, attribute, action, location string }{
		{"path", "a?b", "          path: \"/items/{attr.a?b}/x\"\n", "endpoints[0].entity.actions.act.path"},
		{"query value", "a?b", "          path: /items/x\n          query: {k: \"{attr.a?b}\"}\n", "endpoints[0].entity.actions.act.query.k"},
		{"body string", "a{b", "          path: /items/x\n          body: {k: \"{attr.a{b}\"}\n", "endpoints[0].entity.actions.act.body.k"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRecipe(recipe(tt.attribute, tt.action))
			want := "placeholder {attr." + tt.attribute + "} names an attribute that cannot be used in a placeholder"
			if err == nil || !strings.Contains(err.Error(), tt.location) || !strings.Contains(err.Error(), want) {
				t.Fatalf("ParseRecipe() error = %v, want %q at %s", err, want, tt.location)
			}
		})
	}
	mapped := recipe("a?b", "          path: /items/x\n")
	if _, err := ParseRecipe(mapped); err != nil {
		t.Fatalf("mapping an attribute named a?b without referencing it was rejected: %v", err)
	}
}
