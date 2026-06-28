// Package custom implements a configurable HTTP connector stub.
package custom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "custom"

func init() {
	connector.Register(connector.TypeSchema{
		Type:     typeName,
		Category: "virtualization",
		Name:     "Custom HTTP",
		Fields: []connector.SchemaField{
			{Key: "url", Label: "Endpoint URL", Type: "text", Required: true, Placeholder: "https://api.example.com/status"},
			{Key: "method", Label: "HTTP Method", Type: "select", Required: false, Default: "GET"},
			{Key: "headers", Label: "Headers (JSON)", Type: "text", Required: false, Placeholder: `{"Authorization": "Bearer token"}`},
		},
	}, func(config map[string]any) (connector.Connector, error) {
		client := &http.Client{Timeout: 30 * time.Second}
		return &CustomConnector{client: client}, nil
	})
}

// CustomConnector is a configurable HTTP connector for custom APIs.
type CustomConnector struct {
	client *http.Client
}

func (c *CustomConnector) Name() string     { return "Custom HTTP" }
func (c *CustomConnector) Type() string     { return typeName }
func (c *CustomConnector) Category() string { return "virtualization" }

func (c *CustomConnector) Validate(ctx context.Context, config map[string]any) error {
	url, _ := config["url"].(string)
	if url == "" {
		return fmt.Errorf("url is required")
	}

	method, _ := config["method"].(string)
	if method == "" {
		method = "GET"
	}

	req, _ := http.NewRequestWithContext(ctx, method, url, nil)
	setHeaders(req, config)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func (c *CustomConnector) Fetch(ctx context.Context, config map[string]any) (*connector.ServiceSnapshot, error) {
	url, _ := config["url"].(string)
	method, _ := config["method"].(string)
	if method == "" {
		method = "GET"
	}

	req, _ := http.NewRequestWithContext(ctx, method, url, nil)
	setHeaders(req, config)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &connector.ServiceSnapshot{
		ServiceName: "Custom: " + url,
		Type:        typeName,
		Sections: []connector.SnapshotSection{
			{Title: "Response", Content: "```json\n" + string(body) + "\n```"},
		},
		Metadata: map[string]string{
			"status_code": fmt.Sprintf("%d", resp.StatusCode),
			"url":         url,
		},
		FetchedAt: time.Now(),
	}, nil
}

func setHeaders(req *http.Request, config map[string]any) {
	if headersRaw, ok := config["headers"]; ok {
		var headers map[string]string
		switch v := headersRaw.(type) {
		case string:
			json.Unmarshal([]byte(v), &headers)
		case map[string]string:
			headers = v
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
}
