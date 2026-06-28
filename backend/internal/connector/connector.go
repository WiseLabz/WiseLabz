// Package connector defines the interface for infrastructure data connectors.
package connector

import (
	"context"
	"time"
)

// Connector fetches infrastructure data from a specific source.
type Connector interface {
	Name() string
	Type() string
	Category() string
	Fetch(ctx context.Context, config map[string]any) (*ServiceSnapshot, error)
	Validate(ctx context.Context, config map[string]any) error
}

// ServiceSnapshot represents a point-in-time view of a service's state.
type ServiceSnapshot struct {
	ServiceName string            `json:"serviceName"`
	Type        string            `json:"type"`
	Sections    []SnapshotSection `json:"sections"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	FetchedAt   time.Time         `json:"fetchedAt"`
}

// SnapshotSection is a named section of infrastructure data.
type SnapshotSection struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Metadata returns a string metadata value, or the fallback if not set.
func (s *ServiceSnapshot) MetadataValue(key, fallback string) string {
	if s.Metadata == nil {
		return fallback
	}
	if v, ok := s.Metadata[key]; ok {
		return v
	}
	return fallback
}
