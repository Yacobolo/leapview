package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/Yacobolo/leapview/pkg/pagestream"
)

const brokerPollInterval = 20 * time.Millisecond

// Broker uses the platform database as a short-lived relay. Public documents
// intentionally have no affinity cookie, so SSE and commands may reach
// different replicas.
type Broker struct {
	db     *sql.DB
	local  *pagestream.Broker
	logger *slog.Logger
	mu     sync.Mutex
	relays map[string]*relaySubscription
}

type relaySubscription struct {
	cancel context.CancelFunc
	refs   int
}

func NewBroker(db *sql.DB, trace *pagestream.TraceStore, logger *slog.Logger) *Broker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Broker{
		db: db, local: pagestream.NewBroker(pagestream.WithTraceStore(trace)), logger: logger,
		relays: map[string]*relaySubscription{},
	}
}

func (b *Broker) TraceStore() *pagestream.TraceStore {
	return b.local.TraceStore()
}

func (b *Broker) Subscribe(streamID string) (<-chan pagestream.SignalPatch, func()) {
	b.mu.Lock()
	var startRelay func()
	if b.relays[streamID] == nil {
		cursor := b.latestEventID(streamID)
		ctx, cancel := context.WithCancel(context.Background())
		b.relays[streamID] = &relaySubscription{cancel: cancel}
		startRelay = func() { go b.relay(ctx, streamID, cursor) }
	}
	b.relays[streamID].refs++
	updates, unsubscribeLocal := b.local.Subscribe(streamID)
	if startRelay != nil {
		startRelay()
	}
	b.mu.Unlock()

	var once sync.Once
	return updates, func() {
		once.Do(func() {
			unsubscribeLocal()
			b.mu.Lock()
			if relay := b.relays[streamID]; relay != nil {
				relay.refs--
				if relay.refs == 0 {
					relay.cancel()
					delete(b.relays, streamID)
				}
			}
			b.mu.Unlock()
		})
	}
}

func (b *Broker) PublishEnvelope(streamID string, envelope pagestream.Envelope) {
	if b == nil || b.db == nil || streamID == "" || len(envelope.Signals) == 0 {
		return
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		b.logger.Error("encode public dashboard stream event", "error", err)
		return
	}
	if _, err := b.db.Exec(`INSERT INTO dashboard_publication_stream_events (stream_id, envelope_json, created_at) VALUES (?, ?, ?)`,
		streamID, string(payload), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		b.logger.Error("publish public dashboard stream event", "error", err)
	}
}

func (b *Broker) latestEventID(streamID string) int64 {
	if b == nil || b.db == nil {
		return 0
	}
	var id int64
	_ = b.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM dashboard_publication_stream_events WHERE stream_id = ?`, streamID).Scan(&id)
	return id
}

func (b *Broker) relay(ctx context.Context, streamID string, cursor int64) {
	ticker := time.NewTicker(brokerPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := b.db.QueryContext(ctx, `SELECT id, envelope_json FROM dashboard_publication_stream_events WHERE stream_id = ? AND id > ? ORDER BY id`, streamID, cursor)
			if err != nil {
				if ctx.Err() == nil {
					b.logger.Warn("read public dashboard stream events", "error", err)
				}
				continue
			}
			for rows.Next() {
				var id int64
				var payload string
				if err := rows.Scan(&id, &payload); err != nil {
					continue
				}
				var envelope pagestream.Envelope
				if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
					b.logger.Warn("decode public dashboard stream event", "error", err)
					cursor = id
					continue
				}
				b.local.PublishEnvelope(streamID, envelope)
				cursor = id
			}
			_ = rows.Close()
		}
	}
}
