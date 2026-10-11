package storage

import (
	"context"
	"log/slog"
	"time"

	"go.uber.org/fx"
)

// sweepInterval and orphanGraceTTL together decide how aggressively the
// sweeper reclaims storage. An hourly sweep is cheap at this scale; the
// 24h grace period is generous enough that an upload mid-flight between
// presign and confirm — or a slow mobile connection — is never caught by a
// sweep that happens to land in the middle of it.
const (
	sweepInterval  = time.Hour
	orphanGraceTTL = 24 * time.Hour
)

// Keys is the one query the cleanup sweeper needs: every storage key any
// family's item_photos/item_attachments rows still reference. Declared
// narrowly here rather than taking db.Querier, the same convention handler
// packages' own Store interfaces follow.
type Keys interface {
	ListAllStorageKeys(ctx context.Context) ([]string, error)
}

// Sweeper deletes bucket objects nothing references any more: uploads that
// were presigned but never confirmed, or an object a best-effort delete
// couldn't remove at the time (SPEC 6, M4's orphaned-object cleanup job).
//
// It runs as an in-process ticker, not a separate cron job — there is no
// scheduler infrastructure in this repo yet, and an hourly in-process sweep
// is the same trade M7's alert engine makes ("no cron needed at this
// scale").
type Sweeper struct {
	client Client
	keys   Keys
	log    *slog.Logger
	cancel context.CancelFunc
}

// NewSweeper builds the sweeper and registers its lifecycle with fx:
// OnStart derives a cancelable context and spawns the ticking goroutine on
// it, OnStop cancels that context to signal the goroutine to exit — rather
// than the goroutine reaching for context.Background() itself, which is
// detached from any shutdown signal.
func NewSweeper(lc fx.Lifecycle, client Client, keys Keys, log *slog.Logger) *Sweeper {
	s := &Sweeper{client: client, keys: keys, log: log.With(slog.String("component", "storage.sweeper"))}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			var ctx context.Context

			ctx, s.cancel = context.WithCancel(context.Background())
			go s.run(ctx)

			return nil
		},
		OnStop: func(context.Context) error {
			s.cancel()

			return nil
		},
	})

	return s
}

func (s *Sweeper) run(ctx context.Context) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep(ctx)
		}
	}
}

// sweep deletes every bucket object that is both unreferenced and older
// than the grace period. Logged and skipped on error rather than retried
// immediately — the next tick tries again.
func (s *Sweeper) sweep(ctx context.Context) {
	referenced, err := s.keys.ListAllStorageKeys(ctx)
	if err != nil {
		s.log.Error("list storage keys", slog.Any("error", err))

		return
	}

	known := make(map[string]struct{}, len(referenced))
	for _, key := range referenced {
		known[key] = struct{}{}
	}

	objects, err := s.client.ListObjects(ctx)
	if err != nil {
		s.log.Error("list bucket objects", slog.Any("error", err))

		return
	}

	cutoff := time.Now().Add(-orphanGraceTTL)

	for _, obj := range objects {
		if _, ok := known[obj.Key]; ok {
			continue
		}

		if obj.LastModified.After(cutoff) {
			continue // might still be mid-upload between presign and confirm
		}

		if err := s.client.DeleteObject(ctx, obj.Key); err != nil {
			s.log.Error("delete orphaned object", slog.String("key", obj.Key), slog.Any("error", err))
		}
	}
}
