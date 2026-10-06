package connector

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ScopedExternalID qualifies an appliance-local key with its API source, so
// unrelated appliances do not become a strong identity match. Changing the
// configured source URL changes the scope; trailing slashes are ignored.
func ScopedExternalID(connectorType, sourceURL, upstreamID string) string {
	source := sha256.Sum256([]byte(strings.TrimRight(sourceURL, "/")))
	return fmt.Sprintf("%s:%x:%s", connectorType, source, upstreamID)
}
