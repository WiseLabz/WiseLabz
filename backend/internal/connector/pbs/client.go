package pbs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func (c *Connector) request(ctx context.Context, path string, query url.Values) ([]byte, error) {
	requestURL := c.url + "/api2/json" + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, errors.New("pbs request could not be created")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "PBSAPIToken="+c.tokenID+":"+c.tokenSecret)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, connector.MapTransportError(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		statusErr := fmt.Errorf("API returned %d", resp.StatusCode)
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, connector.NewAuthError(statusErr)
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return nil, connector.NewServiceUnavailableError(statusErr)
		default:
			return nil, statusErr
		}
	}

	data, err := connector.ReadBody(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func decodeData(raw []byte, out any) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return connector.NewMalformedResponseError(errors.New("invalid response"))
	}
	if envelope.Data == nil {
		return connector.NewMalformedResponseError(errors.New("invalid response"))
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return connector.NewMalformedResponseError(errors.New("invalid response"))
	}
	return nil
}

func (c *Connector) listDatastores(ctx context.Context) ([]datastore, error) {
	body, err := c.request(ctx, "/admin/datastore", nil)
	if err != nil {
		return nil, err
	}
	var stores []datastore
	if err := decodeData(body, &stores); err != nil {
		return nil, err
	}
	return stores, nil
}

func (c *Connector) listNamespaces(ctx context.Context, store string) ([]string, error) {
	escapedStore := url.PathEscape(store)
	body, err := c.request(ctx, "/admin/datastore/"+escapedStore+"/namespace", nil)
	if err != nil {
		return nil, err
	}
	var nsInfos []namespaceInfo
	if err := decodeData(body, &nsInfos); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	seen[""] = true
	for _, info := range nsInfos {
		seen[info.NS] = true
	}

	result := make([]string, 0, len(seen))
	for ns := range seen {
		result = append(result, ns)
	}
	sort.Strings(result)
	return result, nil
}

func (c *Connector) listGroups(ctx context.Context, store, ns string) ([]backupGroup, error) {
	escapedStore := url.PathEscape(store)
	query := url.Values{}
	if ns != "" {
		query.Set("ns", ns)
	}

	body, err := c.request(ctx, "/admin/datastore/"+escapedStore+"/groups", query)
	if err != nil {
		return nil, err
	}
	var groups []backupGroup
	if err := decodeData(body, &groups); err != nil {
		return nil, err
	}

	for i := range groups {
		groups[i].Store = store
		groups[i].Namespace = ns
	}
	return groups, nil
}

func (c *Connector) listVerifyJobs(ctx context.Context) ([]verifyJob, error) {
	body, err := c.request(ctx, "/config/verify", nil)
	if err != nil {
		return nil, err
	}
	var jobs []verifyJob
	if err := decodeData(body, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (c *Connector) listPruneJobs(ctx context.Context) ([]pruneJob, error) {
	body, err := c.request(ctx, "/config/prune", nil)
	if err != nil {
		return nil, err
	}
	var jobs []pruneJob
	if err := decodeData(body, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (c *Connector) newestVerifyState(ctx context.Context, g backupGroup) (string, error) {
	query := url.Values{}
	query.Set("backup-type", g.Type)
	query.Set("backup-id", string(g.ID))
	if g.Namespace != "" {
		query.Set("ns", g.Namespace)
	}

	escapedStore := url.PathEscape(g.Store)
	body, err := c.request(ctx, "/admin/datastore/"+escapedStore+"/snapshots", query)
	if err != nil {
		return "", err
	}
	var snapshots []snapshotInfo
	if err := decodeData(body, &snapshots); err != nil {
		return "", err
	}

	if len(snapshots) == 0 {
		return "none", nil
	}

	newest := &snapshots[0]
	for i := range snapshots {
		if snapshots[i].BackupTime > newest.BackupTime {
			newest = &snapshots[i]
		}
	}

	if newest.Verification == nil {
		return "none", nil
	}
	switch newest.Verification.State {
	case "ok", "failed":
		return newest.Verification.State, nil
	}
	return "none", nil
}
