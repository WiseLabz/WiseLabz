package proxmox

import (
	"context"
	"io"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func (p *Connector) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	url := p.url + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Authorization", "PVEAPIToken="+p.tokenID+"="+p.tokenSecret)
	req.Header.Set("Accept", "application/json")

	return connector.Do(p.client, req)
}
