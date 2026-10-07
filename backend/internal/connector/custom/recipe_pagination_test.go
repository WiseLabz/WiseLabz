package custom

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func paginationRecipe(auth, pagination, extra string) string {
	endpoint := strings.Replace(fetchEndpoint, "    method: GET", "    method: GET\n"+extra, 1)
	endpoint = strings.Replace(endpoint, "    entity:", "    pagination: "+pagination+"\n    entity:", 1)
	return recipeForFetch(auth, endpoint)
}

func fetchPaginationRecipe(ctx context.Context, serverURL, auth, pagination, extra string, credentials map[string]any, client *http.Client) (*connector.ServiceSnapshot, error) {
	config := map[string]any{
		"url":    serverURL,
		"recipe": paginationRecipe(auth, pagination, extra),
	}
	for key, value := range credentials {
		config[key] = value
	}
	return (&Connector{client: client}).Fetch(ctx, config)
}

func TestFetchRecipePageAndOffsetPaginationStopOnEmptyItems(t *testing.T) {
	tests := []struct {
		name       string
		pagination string
		param      string
		values     []string
	}{
		{name: "page", pagination: "{type: page, param: page, size: 2}", param: "page", values: []string{"1", "2"}},
		{name: "offset", pagination: "{type: offset, param: offset, size_param: limit, size: 2}", param: "offset", values: []string{"0", "2"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := int(calls.Add(1))
				if got := r.URL.Query().Get(test.param); got != test.values[call-1] {
					t.Errorf("%s = %q, want %q", test.param, got, test.values[call-1])
				}
				if call == 1 {
					_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
					return
				}
				_, _ = io.WriteString(w, `{"items":[]}`)
			}))
			defer server.Close()

			snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", test.pagination, "", nil, server.Client())
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if calls.Load() != 2 || len(snapshot.Entities) != 1 || snapshot.Metadata["endpoint.items.items"] != "1" {
				t.Fatalf("requests/entities/items = %d/%d/%s", calls.Load(), len(snapshot.Entities), snapshot.Metadata["endpoint.items.items"])
			}
		})
	}
}

func TestFetchRecipeCursorPaginationStopsOnAbsentOrEmptyCursor(t *testing.T) {
	for _, test := range []struct {
		name  string
		field string
	}{
		{name: "absent"},
		{name: "empty", field: `,"nextCursor":""`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]`+test.field+`}`)
			}))
			defer server.Close()

			snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: cursor, param: cursor, cursor_path: nextCursor}", "", nil, server.Client())
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if calls.Load() != 1 || len(snapshot.Entities) != 1 {
				t.Fatalf("requests/entities = %d/%d, want 1/1", calls.Load(), len(snapshot.Entities))
			}
		})
	}
}

func TestFetchRecipeCursorPaginationFailsWhenCursorRepeats(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call > 1 && r.URL.Query().Get("cursor") != "next" {
			t.Errorf("cursor query = %q, want next", r.URL.Query().Get("cursor"))
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"`+fmt.Sprint(call)+`","name":"item"}],"nextCursor":"next"}`)
	}))
	defer server.Close()

	snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: cursor, param: cursor, cursor_path: nextCursor}", "", nil, server.Client())
	if snapshot != nil || calls.Load() != 2 || err == nil || !strings.Contains(err.Error(), "cursor did not advance") {
		t.Fatalf("Fetch() = %#v, requests %d, error %v; want repeated cursor failure after two requests", snapshot, calls.Load(), err)
	}
}

func TestFetchRecipeCursorPaginationDetectsInitialCursorRepeat(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if got := r.URL.Query().Get("cursor"); got != "initial" {
			t.Errorf("cursor = %q, want initial", got)
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}],"nextCursor":"initial"}`)
	}))
	defer server.Close()

	snapshot, err := fetchPaginationRecipe(
		context.Background(),
		server.URL,
		"  mode: none",
		"{type: cursor, param: cursor, cursor_path: nextCursor}",
		"    query: {cursor: initial}\n",
		nil,
		server.Client(),
	)
	if snapshot != nil || calls.Load() != 1 || err == nil || !strings.Contains(err.Error(), "cursor did not advance") {
		t.Fatalf("Fetch() = %#v, requests %d, error %v; want initial cursor repeat failure", snapshot, calls.Load(), err)
	}
}

func TestFetchRecipeCursorPaginationAppliesOptionalSize(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Errorf("limit = %q, want 2", got)
		}
		if call == 1 && r.URL.Query().Get("cursor") != "initial" {
			t.Errorf("initial cursor = %q, want initial", r.URL.Query().Get("cursor"))
		}
		if call == 2 && r.URL.Query().Get("cursor") != "next" {
			t.Errorf("next cursor = %q, want next", r.URL.Query().Get("cursor"))
		}
		if call == 1 {
			_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}],"nextCursor":"next"}`)
			return
		}
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	defer server.Close()

	snapshot, err := fetchPaginationRecipe(
		context.Background(),
		server.URL,
		"  mode: none",
		"{type: cursor, param: cursor, cursor_path: nextCursor, size_param: limit, size: 2}",
		"    query: {cursor: initial}\n",
		nil,
		server.Client(),
	)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if calls.Load() != 2 || len(snapshot.Entities) != 1 {
		t.Fatalf("requests/entities = %d/%d, want 2/1", calls.Load(), len(snapshot.Entities))
	}
}

func TestFetchRecipeNextLinkFromBodyStopsWhenAbsentOrEmpty(t *testing.T) {
	for _, test := range []struct {
		name  string
		field string
	}{
		{name: "absent"},
		{name: "empty", field: `,"paging":{"next":""}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]`+test.field+`}`)
			}))
			defer server.Close()

			snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: next_link, next_path: paging.next}", "", nil, server.Client())
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if calls.Load() != 1 || len(snapshot.Entities) != 1 {
				t.Fatalf("requests/entities = %d/%d, want 1/1", calls.Load(), len(snapshot.Entities))
			}
		})
	}
}

func TestFetchRecipeNextLinkBodyRequestsSameOriginAndDetectsRepeat(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call == 2 && r.URL.Query().Get("page") != "2" {
			t.Errorf("page = %q, want 2", r.URL.Query().Get("page"))
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"`+fmt.Sprint(call)+`","name":"item"}],"paging":{"next":"/api/items?page=2"}}`)
	}))
	defer server.Close()

	snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: next_link, next_path: paging.next}", "", nil, server.Client())
	if snapshot != nil || calls.Load() != 2 || err == nil || !strings.Contains(err.Error(), "next link repeated") {
		t.Fatalf("Fetch() = %#v, requests %d, error %v; want repeated link failure after two requests", snapshot, calls.Load(), err)
	}
}

func TestFetchRecipeResolvesRelativeNextLinkAgainstConnectorURL(t *testing.T) {
	for _, test := range []struct {
		name string
		link string
		path string
	}{
		{name: "relative path", link: "next?page=2", path: "/root/next"},
		{name: "query only", link: "?page=2", path: "/root/"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := calls.Add(1)
				if call == 2 && r.URL.Path != test.path {
					t.Errorf("next path = %q, want %s", r.URL.Path, test.path)
				}
				if call == 1 {
					_, _ = fmt.Fprintf(w, `{"items":[{"id":"1","name":"one"}],"paging":{"next":%q}}`, test.link)
					return
				}
				_, _ = io.WriteString(w, `{"items":[]}`)
			}))
			defer server.Close()

			snapshot, err := fetchPaginationRecipe(context.Background(), server.URL+"/root/", "  mode: none", "{type: next_link, next_path: paging.next}", "", nil, server.Client())
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if calls.Load() != 2 || len(snapshot.Entities) != 1 {
				t.Fatalf("requests/entities = %d/%d, want 2/1", calls.Load(), len(snapshot.Entities))
			}
		})
	}
}

func TestFetchRecipeLinkHeaderParsesMultipleRelationsAndQuotedCommas(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call == 2 && (r.URL.Path != "/api/items" || r.URL.Query().Get("page") != "2") {
			t.Errorf("next request URL = %s, want /api/items?page=2", r.URL.String())
		}
		if call == 1 {
			w.Header().Add("Link", `</previous>; rel="prev", </api/items?page=2>; title="page, two"; rel="prev next"`)
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"`+fmt.Sprint(call)+`","name":"item"}]}`)
	}))
	defer server.Close()

	snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: next_link, link_header: true}", "", nil, server.Client())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if calls.Load() != 2 || len(snapshot.Entities) != 2 {
		t.Fatalf("requests/entities = %d/%d, want 2/2", calls.Load(), len(snapshot.Entities))
	}
}

func TestFetchRecipeMalformedLinkHeadersStopWithoutPanic(t *testing.T) {
	for _, value := range []string{
		`</api/items?page=2>; rel="`,
		`</api/items?page=2>; rel="next`,
		`malformed link value`,
	} {
		t.Run(value, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Link", value)
				_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
			}))
			defer server.Close()

			snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: next_link, link_header: true}", "", nil, server.Client())
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if calls.Load() != 1 || len(snapshot.Entities) != 1 {
				t.Fatalf("requests/entities = %d/%d, want 1/1", calls.Load(), len(snapshot.Entities))
			}
		})
	}
}

func TestFetchRecipeLinkHeaderWithoutNextRelationStops(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Link", `</api/items?page=2>; rel="prev alternate"`)
		_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
	}))
	defer server.Close()

	snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: next_link, link_header: true}", "", nil, server.Client())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if calls.Load() != 1 || len(snapshot.Entities) != 1 {
		t.Fatalf("requests/entities = %d/%d, want 1/1", calls.Load(), len(snapshot.Entities))
	}
}

func TestFetchRecipeRejectsCrossOriginNextLinkBeforeRequest(t *testing.T) {
	for _, source := range []string{"body", "Link header"} {
		t.Run(source, func(t *testing.T) {
			var targetCalls atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				targetCalls.Add(1)
				_, _ = io.WriteString(w, `{"items":[]}`)
			}))
			defer target.Close()

			var sourceCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				sourceCalls.Add(1)
				link := target.URL + "/items?api_token=link-secret&api_token=duplicate"
				if source == "Link header" {
					w.Header().Set("Link", "<"+link+">; rel=next")
					_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
					return
				}
				_, _ = fmt.Fprintf(w, `{"items":[{"id":"1","name":"one"}],"paging":{"next":%q}}`, link)
			}))
			defer server.Close()

			pagination := "{type: next_link, next_path: paging.next}"
			if source == "Link header" {
				pagination = "{type: next_link, link_header: true}"
			}
			snapshot, err := fetchPaginationRecipe(
				context.Background(),
				server.URL,
				"  mode: query\n  name: api_token",
				pagination,
				"",
				map[string]any{"auth_token": "connector-secret"},
				server.Client(),
			)
			if snapshot != nil || sourceCalls.Load() != 1 || targetCalls.Load() != 0 || err == nil || !strings.Contains(err.Error(), `endpoint "items"`) || !strings.Contains(err.Error(), "outside connector origin") || strings.Contains(err.Error(), "connector-secret") || strings.Contains(err.Error(), "link-secret") || strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("Fetch() = %#v, source/target requests %d/%d, error %v; want safe cross-origin failure", snapshot, sourceCalls.Load(), targetCalls.Load(), err)
			}
		})
	}
}

func TestFetchRecipePaginationReappliesAuthenticationOnEveryPage(t *testing.T) {
	tests := []struct {
		name        string
		auth        string
		credentials map[string]any
		check       func(*testing.T, *http.Request)
	}{
		{
			name:        "header and legacy",
			auth:        "  mode: header\n  name: X-Api-Key",
			credentials: map[string]any{"auth_token": "header-token", "headers": `{"X-Legacy":"legacy"}`},
			check: func(t *testing.T, r *http.Request) {
				if got := r.Header.Get("X-Api-Key"); got != "header-token" {
					t.Errorf("X-Api-Key = %q", got)
				}
				if got := r.Header.Get("X-Legacy"); got != "legacy" {
					t.Errorf("X-Legacy = %q", got)
				}
			},
		},
		{
			name:        "basic",
			auth:        "  mode: basic",
			credentials: map[string]any{"auth_username": "operator", "auth_password": "basic-secret"},
			check: func(t *testing.T, r *http.Request) {
				user, password, ok := r.BasicAuth()
				if !ok || user != "operator" || password != "basic-secret" {
					t.Errorf("BasicAuth() = %q, %q, %t", user, password, ok)
				}
			},
		},
		{
			name:        "query only once",
			auth:        "  mode: query\n  name: api_token",
			credentials: map[string]any{"auth_token": "query-token"},
			check: func(t *testing.T, r *http.Request) {
				values := r.URL.Query()["api_token"]
				if len(values) != 1 || values[0] != "query-token" {
					t.Errorf("api_token values = %q", values)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				test.check(t, r)
				if calls.Add(1) == 1 {
					_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
					return
				}
				_, _ = io.WriteString(w, `{"items":[]}`)
			}))
			defer server.Close()

			snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, test.auth, "{type: page, param: page, size: 100}", "", test.credentials, server.Client())
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if calls.Load() != 2 || len(snapshot.Entities) != 1 {
				t.Fatalf("requests/entities = %d/%d, want 2/1", calls.Load(), len(snapshot.Entities))
			}
		})
	}
}

func TestFetchRecipeNextLinkQueryAuthOverridesLinkTokensOnce(t *testing.T) {
	for _, source := range []string{"body", "Link header"} {
		t.Run(source, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := calls.Add(1)
				values := r.URL.Query()["api_token"]
				if len(values) != 1 || values[0] != "connector-secret" {
					t.Errorf("api_token values on request %d = %q, want one connector token", call, values)
				}
				if call == 1 {
					link := "/api/items?page=2&api_token=link-secret&api_token=duplicate"
					if source == "Link header" {
						w.Header().Set("Link", "<"+link+">; rel=next")
						_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
						return
					}
					_, _ = fmt.Fprintf(w, `{"items":[{"id":"1","name":"one"}],"paging":{"next":%q}}`, link)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, "request failed")
			}))
			defer server.Close()

			pagination := "{type: next_link, next_path: paging.next}"
			if source == "Link header" {
				pagination = "{type: next_link, link_header: true}"
			}
			snapshot, err := fetchPaginationRecipe(
				context.Background(),
				server.URL,
				"  mode: query\n  name: api_token",
				pagination,
				"",
				map[string]any{"auth_token": "connector-secret"},
				server.Client(),
			)
			if snapshot != nil || calls.Load() != 2 || err == nil || strings.Contains(err.Error(), "connector-secret") || strings.Contains(err.Error(), "link-secret") || strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("Fetch() = %#v, requests %d, error %v; want safe failure after authenticated next request", snapshot, calls.Load(), err)
			}
		})
	}
}

type cancelOnCloseBody struct {
	reader io.Reader
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Read(p []byte) (int, error) { return b.reader.Read(p) }

func (b *cancelOnCloseBody) Close() error {
	b.cancel()
	return nil
}

func TestFetchRecipePaginationHonorsCancellationBetweenPages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       &cancelOnCloseBody{reader: strings.NewReader(`{"items":[{"id":"1","name":"one"}],"nextCursor":"next"}`), cancel: cancel},
		}, nil
	})}

	snapshot, err := fetchPaginationRecipe(ctx, "http://api.example.test", "  mode: none", "{type: cursor, param: cursor, cursor_path: nextCursor}", "", nil, client)
	if snapshot != nil || calls.Load() != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch() = %#v, requests %d, error %v; want canceled between pages after one request", snapshot, calls.Load(), err)
	}
}

func TestFetchRecipePaginationHonorsCancellationDuringRequest(t *testing.T) {
	started := make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}],"nextCursor":"next"}`)
			return
		}
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := fetchPaginationRecipe(ctx, server.URL, "  mode: none", "{type: cursor, param: cursor, cursor_path: nextCursor}", "", nil, server.Client())
		done <- err
	}()
	<-started
	cancel()
	err := <-done
	if calls.Load() != 2 || !errors.Is(err, context.Canceled) {
		t.Fatalf("requests/error = %d/%v; want cancellation during second request", calls.Load(), err)
	}
}

func TestFetchRecipePaginationCapsFailWithoutSnapshot(t *testing.T) {
	t.Run("100 pages", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page := calls.Add(1)
			if got := r.URL.Query().Get("page"); got != fmt.Sprint(page) {
				t.Errorf("page = %q, want %d", got, page)
			}
			_, _ = io.WriteString(w, `{"items":[{"id":"`+fmt.Sprint(page)+`","name":"item"}]}`)
		}))
		defer server.Close()

		snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: page, param: page, size: 1}", "", nil, server.Client())
		if snapshot != nil || calls.Load() != maxRecipePages+1 || err == nil || !strings.Contains(err.Error(), "100-page limit") {
			t.Fatalf("Fetch() = %#v, requests %d, error %v; want 100-page failure after exactly one probe request", snapshot, calls.Load(), err)
		}
	})

	t.Run("10000 entities", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			_, _ = io.WriteString(w, `{"items":[`)
			for i := 0; i <= maxRecipeEntities; i++ {
				if i > 0 {
					_, _ = io.WriteString(w, ",")
				}
				_, _ = fmt.Fprintf(w, `{"id":"%d","name":"item"}`, i)
			}
			_, _ = io.WriteString(w, `]}`)
		}))
		defer server.Close()

		snapshot, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
			"url":    server.URL,
			"recipe": recipeForFetch("  mode: none", fetchEndpoint),
		})
		if snapshot != nil || calls.Load() != 1 || err == nil || !strings.Contains(err.Error(), "10000-entity limit") {
			t.Fatalf("Fetch() = %#v, requests %d, error %v; want entity-limit failure without snapshot", snapshot, calls.Load(), err)
		}
	})
}

func TestFetchRecipeEntityCapAccumulatesAcrossPagesAndEndpoints(t *testing.T) {
	firstEndpoint := strings.Replace(fetchEndpoint, "/api/items", "/first", 1)
	firstEndpoint = strings.Replace(firstEndpoint, "    entity:", "    pagination: {type: page, param: page, size: 3000}\n    entity:", 1)
	secondEndpoint := strings.Replace(fetchEndpoint, "/api/items", "/second", 1)
	secondEndpoint = strings.Replace(secondEndpoint, "name: items", "name: second", 1)
	endpoints := firstEndpoint + secondEndpoint
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/first":
			switch r.URL.Query().Get("page") {
			case "1", "2":
				if err := writePaginationItems(w, "first", 3000, r.URL.Query().Get("page")); err != nil {
					t.Error(err)
				}
			default:
				_, _ = io.WriteString(w, `{"items":[]}`)
			}
		case "/second":
			if err := writePaginationItems(w, "second", 4001, ""); err != nil {
				t.Error(err)
			}
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	recipe := recipeForFetch("  mode: none", endpoints)
	snapshot, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{"url": server.URL, "recipe": recipe})
	if snapshot != nil || calls.Load() != 4 || err == nil || !strings.Contains(err.Error(), "10000-entity limit") || !strings.Contains(err.Error(), `endpoint "second"`) {
		t.Fatalf("Fetch() = %#v, requests %d, error %v; want cumulative endpoint cap failure", snapshot, calls.Load(), err)
	}
}

func TestFetchRecipePaginationKeepsWholeSyncMappedOutputBudget(t *testing.T) {
	largeName := strings.Repeat("x", 6<<20)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		page := calls.Add(1)
		if page == 1 {
			_, _ = fmt.Fprintf(w, `{"items":[{"id":"1","name":%q}]}`, largeName)
			return
		}
		_, _ = fmt.Fprintf(w, `{"items":[{"id":"2","name":%q}]}`, largeName)
	}))
	defer server.Close()

	snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: page, param: page, size: 1}", "", nil, server.Client())
	wantLimit := fmt.Sprintf("maps more than %d bytes", connector.MaxResponseBytes)
	if snapshot != nil || calls.Load() != 2 || err == nil || !strings.Contains(err.Error(), wantLimit) {
		t.Fatalf("Fetch() = %#v, requests %d, error %v; want cumulative output-budget failure reporting %q", snapshot, calls.Load(), err, wantLimit)
	}
}

func TestFetchRecipeMappedOutputBudgetAccumulatesAcrossEndpoints(t *testing.T) {
	firstName := strings.Repeat("a", 6<<20)
	secondName := strings.Repeat("b", 5<<20)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		name := firstName
		if r.URL.Path == "/second" {
			name = secondName
		}
		_, _ = fmt.Fprintf(w, `{"items":[{"id":"%s","name":%q}]}`, r.URL.Path, name)
	}))
	defer server.Close()

	firstEndpoint := strings.Replace(fetchEndpoint, "/api/items", "/first", 1)
	secondEndpoint := strings.Replace(fetchEndpoint, "/api/items", "/second", 1)
	secondEndpoint = strings.Replace(secondEndpoint, "name: items", "name: second", 1)
	snapshot, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url":    server.URL,
		"recipe": recipeForFetch("  mode: none", firstEndpoint+secondEndpoint),
	})
	if snapshot != nil || calls.Load() != 2 || err == nil || !strings.Contains(err.Error(), "maps more than") {
		t.Fatalf("Fetch() = %#v, requests %d, error %v; want cumulative output-budget failure", snapshot, calls.Load(), err)
	}
}

func writePaginationItems(w io.Writer, prefix string, count int, page string) error {
	if _, err := io.WriteString(w, `{"items":[`); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, `{"id":"%s-%s-%d","name":"item"}`, prefix, page, i); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, `]}`)
	return err
}

func TestDocumentedPaginationRecipesParse(t *testing.T) {
	document, err := os.ReadFile("../../../../docs/connectors/RECIPE_FORMAT.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(document), "<!-- pagination-recipes-start -->")
	if !found {
		t.Fatal("missing pagination recipes start marker")
	}
	section, _, found = strings.Cut(section, "<!-- pagination-recipes-end -->")
	if !found {
		t.Fatal("missing pagination recipes end marker")
	}
	count := 0
	for {
		_, section, found = strings.Cut(section, "```yaml\n")
		if !found {
			break
		}
		snippet, rest, found := strings.Cut(section, "```")
		if !found {
			t.Fatal("unterminated YAML recipe snippet")
		}
		if _, err := ParseRecipe(snippet); err != nil {
			t.Errorf("documented pagination recipe %d: %v", count+1, err)
		}
		count++
		section = rest
	}
	if count != 5 {
		t.Fatalf("parsed %d pagination recipes, want 5", count)
	}
}

func TestFetchRecipePaginationEndProbeAfterLastAllowedPage(t *testing.T) {
	t.Run("page mode with exactly 100 non-empty pages succeeds", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			page := calls.Add(1)
			if page > maxRecipePages {
				_, _ = io.WriteString(w, `{"items":[]}`)
				return
			}
			_, _ = io.WriteString(w, `{"items":[{"id":"`+fmt.Sprint(page)+`","name":"item"}]}`)
		}))
		defer server.Close()

		snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: page, param: page, size: 1}", "", nil, server.Client())
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		if calls.Load() != maxRecipePages+1 || len(snapshot.Entities) != maxRecipePages {
			t.Fatalf("requests/entities = %d/%d, want %d/%d", calls.Load(), len(snapshot.Entities), maxRecipePages+1, maxRecipePages)
		}
	})

	t.Run("cursor mode ending on page 100 sends no probe", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			page := calls.Add(1)
			cursor := ""
			if page < maxRecipePages {
				cursor = fmt.Sprintf(`,"nextCursor":"c%d"`, page)
			}
			_, _ = io.WriteString(w, `{"items":[{"id":"`+fmt.Sprint(page)+`","name":"item"}]`+cursor+`}`)
		}))
		defer server.Close()

		snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: cursor, param: cursor, cursor_path: nextCursor}", "", nil, server.Client())
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		if calls.Load() != maxRecipePages || len(snapshot.Entities) != maxRecipePages {
			t.Fatalf("requests/entities = %d/%d, want %d/%d", calls.Load(), len(snapshot.Entities), maxRecipePages, maxRecipePages)
		}
	})

	t.Run("empty pages that keep announcing a next page are bounded", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			page := calls.Add(1)
			_, _ = fmt.Fprintf(w, `{"items":[],"nextCursor":"c%d"}`, page)
		}))
		defer server.Close()

		snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: cursor, param: cursor, cursor_path: nextCursor}", "", nil, server.Client())
		if snapshot != nil || calls.Load() != maxRecipePages+1 || err == nil || !strings.Contains(err.Error(), "100-page limit") {
			t.Fatalf("Fetch() = %#v, requests %d, error %v; want 100-page failure after %d requests", snapshot, calls.Load(), err, maxRecipePages+1)
		}
	})
}

func TestFetchRecipeRejectsOversizedPaginationValues(t *testing.T) {
	oversized := strings.Repeat("c", maxRecipePaginationValueBytes+1)
	tests := []struct {
		name       string
		pagination string
		serve      func(http.ResponseWriter)
		want       string
	}{
		{
			name:       "cursor",
			pagination: "{type: cursor, param: cursor, cursor_path: nextCursor}",
			serve: func(w http.ResponseWriter) {
				_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}],"nextCursor":"`+oversized+`"}`)
			},
			want: "cursor exceeds",
		},
		{
			name:       "next link from body",
			pagination: "{type: next_link, next_path: paging.next}",
			serve: func(w http.ResponseWriter) {
				link := "/api/items?pad=" + strings.Repeat("a", maxRecipePaginationValueBytes+1-len("/api/items?pad="))
				_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}],"paging":{"next":"`+link+`"}}`)
			},
			want: "next link exceeds",
		},
		{
			name:       "next link from Link header",
			pagination: "{type: next_link, link_header: true}",
			serve: func(w http.ResponseWriter) {
				link := "/api/items?pad=" + strings.Repeat("a", maxRecipePaginationValueBytes+1-len("/api/items?pad="))
				w.Header().Set("Link", "<"+link+`>; rel="next"`)
				_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
			},
			want: "next link exceeds",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				test.serve(w)
			}))
			defer server.Close()

			snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", test.pagination, "", nil, server.Client())
			if snapshot != nil || calls.Load() != 1 || err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "8192-byte limit") {
				t.Fatalf("Fetch() = %#v, requests %d, error %v; want %q failure after one request", snapshot, calls.Load(), err, test.want)
			}
			if strings.Contains(err.Error(), "ccccc") || strings.Contains(err.Error(), "aaaaa") {
				t.Fatalf("error includes the pagination value: %.200s", err)
			}
		})
	}

	t.Run("cursor at the limit is followed", func(t *testing.T) {
		atLimit := strings.Repeat("c", maxRecipePaginationValueBytes)
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}],"nextCursor":"`+atLimit+`"}`)
				return
			}
			if got := r.URL.Query().Get("cursor"); got != atLimit {
				t.Errorf("cursor length = %d, want %d", len(got), len(atLimit))
			}
			_, _ = io.WriteString(w, `{"items":[]}`)
		}))
		defer server.Close()

		snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: cursor, param: cursor, cursor_path: nextCursor}", "", nil, server.Client())
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		if calls.Load() != 2 || len(snapshot.Entities) != 1 {
			t.Fatalf("requests/entities = %d/%d, want 2/1", calls.Load(), len(snapshot.Entities))
		}
	})
}

func TestBuildRecipeRequestForNextLinkOriginAndNormalization(t *testing.T) {
	base, err := recipeBaseURL(map[string]any{"url": "https://api.example.test/root/"})
	if err != nil {
		t.Fatal(err)
	}
	recipe := &Recipe{Auth: RecipeAuth{Mode: "none"}}
	endpoint := RecipeEndpoint{Name: "items", Path: "/api/items", Method: "GET"}

	for _, link := range []string{
		"https://evil.example.test/items",
		"//evil.example.test/items",
		"http://api.example.test/items",
		"https://api.example.test:8443/items",
		"https://api.example.test./items",
		"https://user@api.example.test/items",
		"https://api.example.test@evil.example.test/items",
		"https:evil.example.test/items",
		"https:///items",
		"https://[::1]/items",
		"/items#frag",
		"/items\r\nX-Injected: 1",
	} {
		t.Run("rejects "+link, func(t *testing.T) {
			request, err := buildRecipeRequestForNextLink(context.Background(), base, recipe, endpoint, nil, link)
			if err == nil || request != nil {
				t.Fatalf("buildRecipeRequestForNextLink(%q) = %v, %v; want rejection", link, request, err)
			}
		})
	}

	for _, link := range []string{
		"https://API.EXAMPLE.TEST:443/items?page=2",
		`/\evil.example.test/items`,
		`\\evil.example.test\items`,
		"/a/../../items",
		"/%2e%2e/%2fevil.example.test",
		"?page=2",
	} {
		t.Run("accepts "+link, func(t *testing.T) {
			request, err := buildRecipeRequestForNextLink(context.Background(), base, recipe, endpoint, nil, link)
			if err != nil {
				t.Fatalf("buildRecipeRequestForNextLink(%q) error = %v", link, err)
			}
			if got := request.URL.Hostname(); !strings.EqualFold(got, "api.example.test") || !sameOrigin(base, request.URL) {
				t.Fatalf("request URL = %s (host %q), want same-origin api.example.test", request.URL, got)
			}
		})
	}
}

func TestFetchRecipeDuplicateExternalIDAcrossPagesFails(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
	}))
	defer server.Close()

	snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: page, param: page, size: 1}", "", nil, server.Client())
	if snapshot != nil || calls.Load() != 2 || err == nil {
		t.Fatalf("Fetch() = %#v, requests %d, error %v; want duplicate failure after two requests", snapshot, calls.Load(), err)
	}
	for _, want := range []string{`endpoint "items"`, "page 2", `duplicate identifier "1"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestFetchRecipeLaterPageFailureIsClassifiedAndDiscardsEarlierPages(t *testing.T) {
	tests := []struct {
		name   string
		status int
		check  func(*testing.T, error)
	}{
		{name: "401", status: http.StatusUnauthorized, check: func(t *testing.T, err error) {
			var target *connector.AuthError
			if !errors.As(err, &target) {
				t.Errorf("error %v is not an AuthError", err)
			}
		}},
		{name: "403", status: http.StatusForbidden, check: func(t *testing.T, err error) {
			var target *connector.AuthError
			if !errors.As(err, &target) {
				t.Errorf("error %v is not an AuthError", err)
			}
		}},
		{name: "503", status: http.StatusServiceUnavailable, check: func(t *testing.T, err error) {
			var target *connector.ServiceUnavailableError
			if !errors.As(err, &target) {
				t.Errorf("error %v is not a ServiceUnavailableError", err)
			}
		}},
		{name: "500", status: http.StatusInternalServerError, check: func(t *testing.T, err error) {
			var auth *connector.AuthError
			var unavailable *connector.ServiceUnavailableError
			if errors.As(err, &auth) || errors.As(err, &unavailable) {
				t.Errorf("error %v is classified, want a plain error", err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
					return
				}
				w.WriteHeader(test.status)
			}))
			defer server.Close()

			snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: page, param: page, size: 1}", "", nil, server.Client())
			if snapshot != nil || calls.Load() != 2 || err == nil {
				t.Fatalf("Fetch() = %#v, requests %d, error %v; want failure on page 2 without snapshot", snapshot, calls.Load(), err)
			}
			if !strings.Contains(err.Error(), `endpoint "items"`) || !strings.Contains(err.Error(), "page 2") {
				t.Errorf("error %q does not name the endpoint and page 2", err)
			}
			test.check(t, err)

			unpaginated := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
			}))
			defer unpaginated.Close()
			_, err = (&Connector{client: unpaginated.Client()}).Fetch(context.Background(), map[string]any{
				"url":    unpaginated.URL,
				"recipe": recipeForFetch("  mode: none", fetchEndpoint),
			})
			if err == nil || !strings.Contains(err.Error(), `endpoint "items"`) || strings.Contains(err.Error(), "page") {
				t.Errorf("unpaginated error = %v, want endpoint-only message", err)
			}
		})
	}
}

func TestFetchRecipeSkippedItemsSumAcrossPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "1":
			_, _ = io.WriteString(w, `{"items":[{"name":"no id"},{"id":"1","name":"one"}]}`)
		case "2":
			_, _ = io.WriteString(w, `{"items":[{"name":"no id either"},{"id":"2","name":"two"}]}`)
		default:
			_, _ = io.WriteString(w, `{"items":[]}`)
		}
	}))
	defer server.Close()

	snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: page, param: page, size: 2}", "", nil, server.Client())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got := snapshot.Metadata["endpoint.items.skipped"]; got != "2" {
		t.Errorf("skipped = %q, want 2", got)
	}
	if got := snapshot.Metadata["endpoint.items.items"]; got != "4" {
		t.Errorf("items = %q, want 4", got)
	}
	if len(snapshot.Entities) != 2 {
		t.Errorf("entities = %d, want 2", len(snapshot.Entities))
	}
}

func TestFetchRecipeNextLinkBodyValueTypes(t *testing.T) {
	for _, value := range []string{"42", "true", `{"href":"/api/items?page=2"}`, `["/api/items?page=2"]`} {
		t.Run(value, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}],"paging":{"next":`+value+`}}`)
			}))
			defer server.Close()

			snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: next_link, next_path: paging.next}", "", nil, server.Client())
			if snapshot != nil || calls.Load() != 1 || err == nil || !strings.Contains(err.Error(), "next link path must resolve to a string") {
				t.Fatalf("Fetch() = %#v, requests %d, error %v; want string-type failure after one request", snapshot, calls.Load(), err)
			}
		})
	}

	t.Run("null stops", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}],"paging":{"next":null}}`)
		}))
		defer server.Close()

		snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: next_link, next_path: paging.next}", "", nil, server.Client())
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		if calls.Load() != 1 || len(snapshot.Entities) != 1 {
			t.Fatalf("requests/entities = %d/%d, want 1/1", calls.Load(), len(snapshot.Entities))
		}
	})
}

func TestFetchRecipePaginationCyclesFail(t *testing.T) {
	t.Run("next links", func(t *testing.T) {
		links := []string{"/api/items?p=a", "/api/items?p=b", "/api/items?p=a"}
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			call := int(calls.Add(1))
			_, _ = fmt.Fprintf(w, `{"items":[{"id":"%d","name":"item"}],"paging":{"next":%q}}`, call, links[call-1])
		}))
		defer server.Close()

		snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: next_link, next_path: paging.next}", "", nil, server.Client())
		if snapshot != nil || calls.Load() != 3 || err == nil || !strings.Contains(err.Error(), "next link repeated") {
			t.Fatalf("Fetch() = %#v, requests %d, error %v; want repeated link failure after three requests", snapshot, calls.Load(), err)
		}
	})

	t.Run("cursors", func(t *testing.T) {
		cursors := []string{"a", "b", "a"}
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			call := int(calls.Add(1))
			_, _ = fmt.Fprintf(w, `{"items":[{"id":"%d","name":"item"}],"nextCursor":%q}`, call, cursors[call-1])
		}))
		defer server.Close()

		snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: cursor, param: cursor, cursor_path: nextCursor}", "", nil, server.Client())
		if snapshot != nil || calls.Load() != 3 || err == nil || !strings.Contains(err.Error(), "cursor repeated") {
			t.Fatalf("Fetch() = %#v, requests %d, error %v; want repeated cursor failure after three requests", snapshot, calls.Load(), err)
		}
	})
}

func TestFetchRecipeEntityCapBoundaryAllowsExactlyMaxEntities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := writePaginationItems(w, "item", maxRecipeEntities, ""); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	snapshot, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url":    server.URL,
		"recipe": recipeForFetch("  mode: none", fetchEndpoint),
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(snapshot.Entities) != maxRecipeEntities {
		t.Fatalf("entities = %d, want %d", len(snapshot.Entities), maxRecipeEntities)
	}
}

func TestFetchRecipePOSTPaginationRepeatsStaticBodyOnEveryPage(t *testing.T) {
	const wantBody = `{"query":"query { items }"}`
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("request %d method = %q, want POST", call, r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("request %d Content-Type = %q, want application/json", call, got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request %d body: %v", call, err)
		}
		if string(body) != wantBody {
			t.Errorf("request %d body = %q, want %q", call, body, wantBody)
		}
		if got := r.URL.Query().Get("page"); got != fmt.Sprint(call) {
			t.Errorf("request %d page = %q, want %d", call, got, call)
		}
		if call <= 2 {
			_, _ = fmt.Fprintf(w, `{"items":[{"id":"%d","name":"item"}]}`, call)
			return
		}
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	defer server.Close()

	endpoints := `  - name: items
    path: /api/items
    method: POST
    body: {query: "query { items }"}
    items: items
    pagination: {type: page, param: page, size: 1}
    entity: {kind: media_item, name: name, external_id: id}
`
	snapshot, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url":    server.URL,
		"recipe": recipeForFetch("  mode: none", endpoints),
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if calls.Load() != 3 || len(snapshot.Entities) != 2 {
		t.Fatalf("requests/entities = %d/%d, want 3/2", calls.Load(), len(snapshot.Entities))
	}
}

func TestFetchRecipePaginationKeepsStaticQueryAndReplacesPaginationParameter(t *testing.T) {
	t.Run("endpoint path and static query", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			call := calls.Add(1)
			query := r.URL.Query()
			if got := query["fixed"]; len(got) != 1 || got[0] != "1" {
				t.Errorf("request %d fixed = %q, want [1]", call, got)
			}
			if got := query["extra"]; len(got) != 1 || got[0] != "x" {
				t.Errorf("request %d extra = %q, want [x]", call, got)
			}
			if got := query["page"]; len(got) != 1 || got[0] != fmt.Sprint(call) {
				t.Errorf("request %d page = %q, want [%d]", call, got, call)
			}
			if call == 1 {
				_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
				return
			}
			_, _ = io.WriteString(w, `{"items":[]}`)
		}))
		defer server.Close()

		endpoints := `  - name: items
    path: /api/items?fixed=1&page=9
    method: GET
    query: {extra: x}
    items: items
    pagination: {type: page, param: page, size: 2}
    entity: {kind: media_item, name: name, external_id: id}
`
		snapshot, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
			"url":    server.URL,
			"recipe": recipeForFetch("  mode: none", endpoints),
		})
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		if calls.Load() != 2 || len(snapshot.Entities) != 1 {
			t.Fatalf("requests/entities = %d/%d, want 2/1", calls.Load(), len(snapshot.Entities))
		}
	})

	t.Run("query auth with cursor pagination", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			call := calls.Add(1)
			if got := r.URL.Query()["api_token"]; len(got) != 1 || got[0] != "query-token" {
				t.Errorf("request %d api_token = %q, want one query-token", call, got)
			}
			if call == 1 {
				_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}],"nextCursor":"next"}`)
				return
			}
			_, _ = io.WriteString(w, `{"items":[]}`)
		}))
		defer server.Close()

		snapshot, err := fetchPaginationRecipe(
			context.Background(),
			server.URL,
			"  mode: query\n  name: api_token",
			"{type: cursor, param: cursor, cursor_path: nextCursor}",
			"",
			map[string]any{"auth_token": "query-token"},
			server.Client(),
		)
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		if calls.Load() != 2 || len(snapshot.Entities) != 1 {
			t.Fatalf("requests/entities = %d/%d, want 2/1", calls.Load(), len(snapshot.Entities))
		}
	})
}

func TestFetchRecipeFollowsNextRelationInSeparateLinkHeaderLines(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call == 1 {
			w.Header().Add("Link", `</api/items?page=0>; rel="prev"`)
			w.Header().Add("Link", `</api/items?page=2>; rel="next"`)
			_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"one"}]}`)
			return
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Errorf("page = %q, want 2", got)
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"2","name":"two"}]}`)
	}))
	defer server.Close()

	snapshot, err := fetchPaginationRecipe(context.Background(), server.URL, "  mode: none", "{type: next_link, link_header: true}", "", nil, server.Client())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if calls.Load() != 2 || len(snapshot.Entities) != 2 {
		t.Fatalf("requests/entities = %d/%d, want 2/2", calls.Load(), len(snapshot.Entities))
	}
}
