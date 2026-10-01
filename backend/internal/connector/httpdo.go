package connector

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Do sends req with client and returns the response body. It is the shared
// tail of every connector request: transport failures go through
// MapTransportError, the body is read under ReadBody's cap, and a status >= 400
// is mapped by CheckStatus.
func Do(client *http.Client, req *http.Request) (data []byte, err error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, MapTransportError(err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	data, err = ReadBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if statusErr := CheckStatus(resp.StatusCode, data); statusErr != nil {
		return nil, statusErr
	}
	return data, nil
}

// DoJSON is Do followed by decoding the body into out. An undecodable body is
// reported as a MalformedResponseError.
func DoJSON(client *http.Client, req *http.Request, out any) error {
	data, err := Do(client, req)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return NewMalformedResponseError(err)
	}
	return nil
}
