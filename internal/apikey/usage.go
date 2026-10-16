package apikey

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/tenant"
)

const usageDrainTimeout = 5 * time.Second

// UsageWorker batches last-used writes on a ticker, so authentication never waits on one.
type UsageWorker struct {
	store    Writer
	interval time.Duration
	log      *slog.Logger

	mu      sync.Mutex
	pending map[uuid.UUID]map[uuid.UUID]time.Time
}

// NewUsageWorker returns a UsageWorker that flushes recorded usage to store every interval.
func NewUsageWorker(store Writer, interval time.Duration, log *slog.Logger) *UsageWorker {
	if log == nil {
		log = slog.Default()
	}
	return &UsageWorker{
		store:    store,
		interval: interval,
		log:      log.With("module", "apikey.usage"),
		pending:  make(map[uuid.UUID]map[uuid.UUID]time.Time),
	}
}

// Record queues the last-used time of orgID's key id for the next flush, keeping the latest per key.
func (w *UsageWorker) Record(orgID, id uuid.UUID, at time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	keys := w.pending[orgID]
	if keys == nil {
		keys = make(map[uuid.UUID]time.Time)
		w.pending[orgID] = keys
	}
	if cur, ok := keys[id]; !ok || at.After(cur) {
		keys[id] = at
	}
}

// Name implements worker.Worker.
func (w *UsageWorker) Name() string { return "apikey-usage" }

// Run implements worker.Worker, flushing on a ticker until ctx is done and once more on the way out.
func (w *UsageWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			out, cancel := context.WithTimeout(context.WithoutCancel(ctx), usageDrainTimeout)
			w.flush(out)
			cancel()
			return ctx.Err()
		case <-ticker.C:
			w.flush(ctx)
		}
	}
}

func (w *UsageWorker) flush(ctx context.Context) {
	w.mu.Lock()
	if len(w.pending) == 0 {
		w.mu.Unlock()
		return
	}
	pending := w.pending
	w.pending = make(map[uuid.UUID]map[uuid.UUID]time.Time)
	w.mu.Unlock()

	for orgID, batch := range pending {
		if err := w.store.TouchLastUsed(tenant.Into(ctx, tenant.Context{OrgID: orgID}), batch); err != nil {
			w.log.ErrorContext(ctx, "apikey usage flush failed", slog.Int("keys", len(batch)), slog.Any("err", err))
		}
	}
}
