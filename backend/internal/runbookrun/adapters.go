package runbookrun

import (
	"context"
	"errors"
	"fmt"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/health"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
	syncengine "github.com/WiseLabz/wiselabz/internal/sync"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// The production collaborators, checked against the executor's interfaces.
var (
	_ Store         = (*store.Store)(nil)
	_ RecoveryStore = (*store.Store)(nil)
	_ Lifecycle     = (*connectors.Handler)(nil)
	_ Syncer        = (*syncengine.Engine)(nil)
	_ Spawner       = (*syncengine.Engine)(nil)
	_ HealthChecker = StoreHealth{}
	_ Grants        = StoreGrants{}
	_ Publisher     = (*ws.Hub)(nil)
	_ Notifier      = (*notifications.Dispatcher)(nil)
)

// StoreGrants checks grants against the database.
type StoreGrants struct {
	Store *store.Store
}

// Operator implements Grants. A user who was deleted or disabled after
// starting the run no longer acts, whatever grants remain. The user is read
// from the writer, like the grant check below, so a just-disabled user does not
// pass until a read replica catches up.
func (g StoreGrants) Operator(ctx context.Context, userID, connectorID string) (connectors.LifecycleActor, bool, error) {
	user, err := g.Store.GetUserByIDFromWriter(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return connectors.LifecycleActor{}, false, nil
	}
	if err != nil {
		return connectors.LifecycleActor{}, false, fmt.Errorf("load acting user: %w", err)
	}
	if user.Disabled {
		return connectors.LifecycleActor{}, false, nil
	}
	ok, err := g.Store.UserHasConnectorRole(ctx, user.ID, connectorID, "operator")
	if err != nil {
		return connectors.LifecycleActor{}, false, fmt.Errorf("check operator grant: %w", err)
	}
	if !ok {
		return connectors.LifecycleActor{}, false, nil
	}
	return connectors.LifecycleActor{UserID: user.ID, InstanceAdmin: user.InstanceAdminRole == "admin"}, true, nil
}

// StoreHealth runs the same connector health check as the scheduled job and
// persists its result.
type StoreHealth struct {
	Store         *store.Store
	EncryptionKey string
}

// CheckHealth implements HealthChecker. One check is bounded by the scheduled
// job's per-check timeout as well as by ctx.
func (h StoreHealth) CheckHealth(ctx context.Context, connectorID string) (string, error) {
	rec, err := h.Store.GetConnector(ctx, connectorID)
	if err != nil {
		return "", fmt.Errorf("load connector: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, health.DefaultCheckTimeout)
	defer cancel()
	result, err := health.RunHealthCheck(ctx, h.Store, rec, h.EncryptionKey)
	if err != nil {
		return "", err
	}
	return result.Status, nil
}
