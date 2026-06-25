# WiseLabz Connector Guide

A connector is a Go package that implements the `Connector` interface. It tells
WiseLabz how to talk to a specific service, what data to fetch, and how to shape
that data into a documentation-ready snapshot.

This guide walks you through building one from scratch.

---

## The Connector interface

Every connector implements this interface, defined in `backend/connector/connector.go`:

```go
package connector

import "context"

type Connector interface {
    Name()     string
    Fetch(ctx context.Context) (ServiceSnapshot, error)
    Validate() error
}
```

| Method       | Purpose                                                                                                                                                                  |
|--------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `Name()`     | Return a stable, human-readable name for this connector instance (e.g. `"home-proxmox"`). Used in logs, the dashboard, and generated doc headings.                       |
| `Fetch(ctx)` | Connect to the service, pull the data you care about, and return a `ServiceSnapshot`. The context carries a deadline — don't ignore it.                                  |
| `Validate()` | Check that the connector's configuration is usable before we attempt a `Fetch`. Return `nil` if everything looks good, or an error describing what's missing or invalid. |

Registration is handled by `init()` — see [Step 4: Register the connector](#4-register-the-connector) below.

## The ServiceSnapshot

`Fetch` returns a `ServiceSnapshot`:

```go
type ServiceSnapshot struct {
    ServiceName string            // matches connector Name()
    Type        string            // e.g. "proxmox", "portainer", "pfsense"
    Sections    []Section         // the documentation sections this service contributes
    Metadata    map[string]string // arbitrary key-value pairs (version, endpoint, etc.)
    FetchedAt   time.Time         // set by the caller, but you can set it if needed
}

type Section struct {
    Title   string // section heading in the generated doc
    Content string // markdown body — tables, lists, code fences are all fine
    Order   int    // sort ordering within the service's doc page
}
```

Guidelines for a good snapshot:

- **One section per logical unit.** For Proxmox, that might be VMs, storage,
  and network — three sections. Don't dump everything into one blob.
- **Content is Markdown.** You control the formatting. Use GFM tables for
  structured data, fenced code blocks for config dumps, and lists for
  enumerations.
- **Metadata is optional but useful.** Stick the service version, API endpoint,
  or connection mode in `Metadata`. It surfaces in the dashboard without
  bloating the generated doc.
- **Keep it read-only by design.** WiseLabz connectors never mutate the
  target service. If your `Fetch` changes state, it's wrong.

## Step by step

### 1. Create the package

```bash
mkdir -p backend/connector/mynewservice
touch backend/connector/mynewservice/mynewservice.go
```

The package name should be the service name, lowercased, no hyphens
(`truenas`, not `true-nas`).

### 2. Define your config struct

```go
package mynewservice

type Config struct {
    Name    string `yaml:"name"`
    URL     string `yaml:"url"`
    APIKey  string `yaml:"-"` // never from YAML — always from env
    VerifyTLS bool  `yaml:"tls_verify"`
}
```

Use `yaml:"-"` for secrets. WiseLabz passes them via environment variables, not
plaintext config files. Document every field in a comment, especially which ones
are required. These comments become the `config.example.yaml` entry for your
connector.

### 3. Implement the interface

```go
type Connector struct {
    cfg    Config
    client *http.Client
}

func New(cfg Config) (*Connector, error) {
    if cfg.APIKey == "" {
        return nil, fmt.Errorf("mynewservice: API key is required")
    }
    return &Connector{
        cfg: cfg,
        client: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{
                    InsecureSkipVerify: !cfg.VerifyTLS,
                },
            },
        },
    }, nil
}

func (c *Connector) Name() string { return c.cfg.Name }

func (c *Connector) Validate() error {
    if c.cfg.URL == "" {
        return fmt.Errorf("mynewservice: URL is required")
    }
    if c.cfg.APIKey == "" {
        return fmt.Errorf("mynewservice: APIKey is required")
    }
    return nil
}

func (c *Connector) Fetch(ctx context.Context) (connector.ServiceSnapshot, error) {
    // 1. Build the request
    req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.URL+"/api/v1/status", nil)
    if err != nil {
        return connector.ServiceSnapshot{}, fmt.Errorf("mynewservice: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

    // 2. Execute
    resp, err := c.client.Do(req)
    if err != nil {
        return connector.ServiceSnapshot{}, fmt.Errorf("mynewservice: fetch failed: %w", err)
    }
    defer resp.Body.Close()

    // 3. Parse into your domain types
    var status MyServiceStatus
    if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
        return connector.ServiceSnapshot{}, fmt.Errorf("mynewservice: decode: %w", err)
    }

    // 4. Shape into a ServiceSnapshot
    return connector.ServiceSnapshot{
        ServiceName: c.cfg.Name,
        Type:        "mynewservice",
        Sections: []connector.Section{
            {
                Title:   "Status",
                Content: formatStatusSection(status),
                Order:   1,
            },
        },
        Metadata: map[string]string{
            "version":  status.Version,
            "endpoint": c.cfg.URL,
        },
        FetchedAt: time.Now(),
    }, nil
}
```

### 4. Register the connector

Create `backend/connector/mynewservice/init.go`:

```go
package mynewservice

import "github.com/WiseLabz/WiseLabz/backend/connector/registry"

func init() {
    registry.Register("mynewservice", func(cfg map[string]any) (connector.Connector, error) {
        // cfg is the deserialized YAML block for this service.
        // Map it into your Config struct.
        var c Config
        // ... mapping logic ...
        return New(c)
    })
}
```

The registry calls your factory during startup for each service entry in `config.yaml`
whose `type` matches the registered string.

### 5. Write tests

```go
func TestConnector_Fetch(t *testing.T) {
    // Start a test HTTP server that mimics your service's API
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"version":"1.2.3","status":"ok"}`))
    }))
    defer srv.Close()

    c, err := New(Config{
        Name:      "test",
        URL:       srv.URL,
        APIKey:    "fake-key",
        VerifyTLS: false,
    })
    require.NoError(t, err)

    snap, err := c.Fetch(context.Background())
    require.NoError(t, err)
    assert.Equal(t, "test", snap.ServiceName)
    assert.Len(t, snap.Sections, 1)
}

func TestConnector_Validate(t *testing.T) {
    _, err := New(Config{})
    require.Error(t, err)
}
```

### 6. Document config fields

Add a commented block to `config.example.yaml` so users know what to set:

```yaml
# Example: MyNewService connector
#   - name: my-newservice
#     type: mynewservice
#     url: https://192.168.1.50:8443
#     tls_verify: false
#     # WISELABZ_SERVICES_N_APIKEY must be set in the environment
```

## Testing without a real instance

If you don't have the service running, you have three options:

1. **Mock HTTP server (recommended).** Use `net/http/httptest` as shown above.
   Capture a real API response once (even from a browser devtools copy-paste),
   save it as a test fixture in `testdata/`, and serve it from the test server.

2. **Interface-based mocking.** If your connector depends on an interface
   (e.g. a `Client` that wraps the HTTP calls), generate a mock with
   `go.uber.org/mock` or write one by hand. Test your business logic without
   touching the network at all.

3. **Integration test with Docker.** If the service has an official Docker
   image, add a `docker compose` service in `deploy/test-fixtures/` that
   starts a throwaway instance. Use `t.Setenv` to point your connector at
   `localhost:<mapped-port>`, then run `Fetch` against a real (but ephemeral)
   service. These tests are tagged `//go:build integration` so `make test`
   skips them by default.

## Conventions

- **Package location:** `backend/connector/<servicename>/`
- **One connector per package.** Don't bundle multiple connectors in one package.
- **Wrapping errors:** Use `fmt.Errorf("servicename: %w", err)` so callers can
  trace which connector failed.
- **Context deadlines:** Don't create a new context inside `Fetch`. Use the one
  passed to you. The caller sets the timeout.
- **Secrets never in config struct tags.** Use `yaml:"-"` and document the
  corresponding `WISELABZ_` env var.
- **Go version and dependencies:** Match the `go.mod` at the repository root.
  Avoid pulling in large dependency trees for simple HTTP calls.

## Getting your connector merged

Once your connector is implemented and tested:

1. Run `make lint` and `make test` to confirm everything passes.
2. Add a row to the Supported Services table in `README.md`.
3. Open a PR. Use the **New connector** type in the pull request template, and
   confirm you've tested against a real instance.
4. A maintainer will review. Expect feedback on error handling, logging, and
   the shape of your snapshot sections.

If you want to request a connector instead of building one, use the
[Connector Request form][connector-request].

[connector-request]: https://github.com/WiseLabz/WiseLabz/issues/new?template=connector_request.yml
