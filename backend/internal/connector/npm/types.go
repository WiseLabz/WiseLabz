package npm

import (
	"bytes"
	"errors"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

type flexBool bool

func (b *flexBool) UnmarshalJSON(raw []byte) error {
	switch string(bytes.TrimSpace(raw)) {
	case "true", "1":
		*b = true
	case "false", "0":
		*b = false
	default:
		return errors.New("boolean must be true, false, 0, or 1")
	}
	return nil
}

// Payload fields follow NPM's backend/schema/components resource objects.
// Credential-bearing meta, access-list items and advanced config are excluded.
type proxyHost struct {
	ID            int      `json:"id"`
	DomainNames   []string `json:"domain_names"`
	ForwardHost   string   `json:"forward_host"`
	ForwardPort   int      `json:"forward_port"`
	ForwardScheme string   `json:"forward_scheme"`
	AccessListID  int      `json:"access_list_id"`
	CertificateID int      `json:"certificate_id"`
	SSLForced     flexBool `json:"ssl_forced"`
	Enabled       flexBool `json:"enabled"`
}

type redirectionHost struct {
	ID                int      `json:"id"`
	DomainNames       []string `json:"domain_names"`
	ForwardHTTPCode   int      `json:"forward_http_code"`
	ForwardScheme     string   `json:"forward_scheme"`
	ForwardDomainName string   `json:"forward_domain_name"`
	PreservePath      flexBool `json:"preserve_path"`
	CertificateID     int      `json:"certificate_id"`
	SSLForced         flexBool `json:"ssl_forced"`
	Enabled           flexBool `json:"enabled"`
}

type stream struct {
	ID             int      `json:"id"`
	IncomingPort   int      `json:"incoming_port"`
	ForwardingHost string   `json:"forwarding_host"`
	ForwardingPort int      `json:"forwarding_port"`
	TCPForwarding  flexBool `json:"tcp_forwarding"`
	UDPForwarding  flexBool `json:"udp_forwarding"`
	CertificateID  int      `json:"certificate_id"`
	Enabled        flexBool `json:"enabled"`
}

type deadHost struct {
	ID            int      `json:"id"`
	DomainNames   []string `json:"domain_names"`
	CertificateID int      `json:"certificate_id"`
	SSLForced     flexBool `json:"ssl_forced"`
	Enabled       flexBool `json:"enabled"`
}

type certificate struct {
	ID          int      `json:"id"`
	Provider    string   `json:"provider"`
	NiceName    string   `json:"nice_name"`
	DomainNames []string `json:"domain_names"`
	ExpiresOn   string   `json:"expires_on"`
}

type accessList struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	SatisfyAny     flexBool `json:"satisfy_any"`
	PassAuth       flexBool `json:"pass_auth"`
	ProxyHostCount int      `json:"proxy_host_count"`
}

type tableBuilder func([]byte) (string, []connector.SnapshotEntity, []connector.ServiceDependency, error)
