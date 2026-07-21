package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Yacobolo/leapview/internal/dashboard"
	"github.com/Yacobolo/leapview/internal/dashboard/command"
	"github.com/Yacobolo/leapview/internal/dashboard/publication"
)

const streamLease = 90 * time.Second
const streamHeartbeat = 30 * time.Second

type StreamRegistry struct {
	mu      sync.Mutex
	streams map[string]map[string]*localStream
	db      *sql.DB
}

type localStream struct {
	cancel         context.CancelFunc
	version        publication.StreamVersion
	registrationID string
}

type streamKey struct {
	publicationID string
	streamID      string
}

func NewStreamRegistry(db *sql.DB) *StreamRegistry {
	return &StreamRegistry{streams: map[string]map[string]*localStream{}, db: db}
}

func (r *StreamRegistry) Register(parent context.Context, publicationID, streamID string, version publication.StreamVersion, initialFilters ...dashboard.Filters) (context.Context, func(), error) {
	ctx, cancel := context.WithCancel(parent)
	registrationID, err := streamRegistrationID()
	if err != nil {
		cancel()
		return ctx, func() {}, err
	}
	filters := dashboard.Filters{}.WithDefaults()
	if len(initialFilters) > 0 {
		filters = initialFilters[0].WithDefaults()
	}
	filtersJSON, err := json.Marshal(filters)
	if err != nil {
		cancel()
		return ctx, func() {}, err
	}
	if _, err := r.db.ExecContext(parent, `INSERT INTO dashboard_publication_streams (
publication_id, stream_id, public_id, serving_state_id, registration_id, filters_json, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(publication_id, stream_id) DO UPDATE SET
public_id = excluded.public_id, serving_state_id = excluded.serving_state_id,
registration_id = excluded.registration_id, filters_json = excluded.filters_json,
generation = 1, expires_at = excluded.expires_at, updated_at = CURRENT_TIMESTAMP`,
		publicationID, streamID, version.PublicID, version.ServingStateID, registrationID, string(filtersJSON), streamExpiry()); err != nil {
		cancel()
		return ctx, func() {}, err
	}
	r.mu.Lock()
	if r.streams[publicationID] == nil {
		r.streams[publicationID] = map[string]*localStream{}
	}
	if previous := r.streams[publicationID][streamID]; previous != nil {
		previous.cancel()
	}
	registration := &localStream{cancel: cancel, version: version, registrationID: registrationID}
	r.streams[publicationID][streamID] = registration
	r.mu.Unlock()
	go r.heartbeat(ctx, publicationID, streamID, registrationID)
	return ctx, func() {
		r.mu.Lock()
		if current := r.streams[publicationID][streamID]; current == registration {
			delete(r.streams[publicationID], streamID)
			if len(r.streams[publicationID]) == 0 {
				delete(r.streams, publicationID)
			}
		}
		r.mu.Unlock()
		cancel()
		_, _ = r.db.ExecContext(context.WithoutCancel(parent), `DELETE FROM dashboard_publication_streams WHERE publication_id = ? AND stream_id = ? AND registration_id = ?`, publicationID, streamID, registrationID)
	}, nil
}

func (r *StreamRegistry) PrepareCommand(ctx context.Context, publicationID, streamID string, version publication.StreamVersion, prepare func(dashboard.Filters) (command.PreparedRefresh, error)) (command.PreparedRefresh, uint64, error) {
	if prepare == nil {
		return command.PreparedRefresh{}, 0, fmt.Errorf("publication command preparation is required")
	}
	for attempt := 0; attempt < 8; attempt++ {
		var filtersJSON string
		var generation uint64
		err := r.db.QueryRowContext(ctx, `SELECT filters_json, generation FROM dashboard_publication_streams
WHERE publication_id = ? AND stream_id = ? AND public_id = ? AND serving_state_id = ? AND expires_at > ?`,
			publicationID, streamID, version.PublicID, version.ServingStateID, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&filtersJSON, &generation)
		if err != nil {
			return command.PreparedRefresh{}, 0, fmt.Errorf("load publication command state: %w", err)
		}
		var filters dashboard.Filters
		if err := json.Unmarshal([]byte(filtersJSON), &filters); err != nil {
			return command.PreparedRefresh{}, 0, fmt.Errorf("decode publication command state: %w", err)
		}
		prepared, err := prepare(filters.WithDefaults())
		if err != nil {
			return command.PreparedRefresh{}, 0, err
		}
		nextFilters, err := json.Marshal(prepared.Filters.WithDefaults())
		if err != nil {
			return command.PreparedRefresh{}, 0, err
		}
		nextGeneration := generation + 1
		result, err := r.db.ExecContext(ctx, `UPDATE dashboard_publication_streams
SET filters_json = ?, generation = ?, expires_at = ?, updated_at = CURRENT_TIMESTAMP
WHERE publication_id = ? AND stream_id = ? AND public_id = ? AND serving_state_id = ? AND generation = ? AND expires_at > ?`,
			string(nextFilters), nextGeneration, streamExpiry(), publicationID, streamID, version.PublicID, version.ServingStateID,
			generation, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return command.PreparedRefresh{}, 0, err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return command.PreparedRefresh{}, 0, err
		}
		if changed == 1 {
			return prepared, nextGeneration, nil
		}
	}
	return command.PreparedRefresh{}, 0, fmt.Errorf("publication command state changed concurrently")
}

func (r *StreamRegistry) Active(publicationID, streamID string, version publication.StreamVersion) bool {
	var exists int
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM dashboard_publication_streams
WHERE publication_id = ? AND stream_id = ? AND public_id = ? AND serving_state_id = ? AND expires_at > ?)`,
		publicationID, streamID, version.PublicID, version.ServingStateID, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&exists)
	return err == nil && exists == 1
}

func (r *StreamRegistry) Reconcile(ctx context.Context, active map[string]publication.StreamVersion) {
	now := time.Now().UTC()
	_, _ = r.db.ExecContext(ctx, `DELETE FROM dashboard_publication_streams WHERE expires_at <= ?`, now.Format(time.RFC3339Nano))
	_, _ = r.db.ExecContext(ctx, `DELETE FROM dashboard_publication_stream_events WHERE created_at <= ?`, now.Add(-10*time.Minute).Format(time.RFC3339Nano))
	durableRegistrations, durableRegistrationsLoaded := r.loadDurableRegistrations(ctx)
	r.mu.Lock()
	stale := []context.CancelFunc{}
	for publicationID, streams := range r.streams {
		current, ok := active[publicationID]
		for streamID, stream := range streams {
			registrationCurrent := true
			if durableRegistrationsLoaded {
				registrationCurrent = durableRegistrations[streamKey{publicationID: publicationID, streamID: streamID}] == stream.registrationID
			}
			if ok && stream.version == current && registrationCurrent {
				continue
			}
			stale = append(stale, stream.cancel)
			delete(streams, streamID)
		}
		if len(streams) == 0 {
			delete(r.streams, publicationID)
		}
	}
	r.mu.Unlock()
	for _, cancel := range stale {
		cancel()
	}
}

func (r *StreamRegistry) loadDurableRegistrations(ctx context.Context) (map[streamKey]string, bool) {
	rows, err := r.db.QueryContext(ctx, `SELECT publication_id, stream_id, registration_id
FROM dashboard_publication_streams WHERE expires_at > ?`, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	registrations := map[streamKey]string{}
	for rows.Next() {
		var key streamKey
		var registrationID string
		if err := rows.Scan(&key.publicationID, &key.streamID, &registrationID); err != nil {
			return nil, false
		}
		registrations[key] = registrationID
	}
	if err := rows.Err(); err != nil {
		return nil, false
	}
	return registrations, true
}

func (r *StreamRegistry) ClosePublication(publicationID string) {
	r.mu.Lock()
	streams := r.streams[publicationID]
	delete(r.streams, publicationID)
	r.mu.Unlock()
	for _, stream := range streams {
		stream.cancel()
	}
	_, _ = r.db.Exec(`DELETE FROM dashboard_publication_streams WHERE publication_id = ?`, publicationID)
}

func (r *StreamRegistry) heartbeat(ctx context.Context, publicationID, streamID, registrationID string) {
	ticker := time.NewTicker(streamHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := r.db.ExecContext(ctx, `UPDATE dashboard_publication_streams SET expires_at = ?, updated_at = CURRENT_TIMESTAMP
WHERE publication_id = ? AND stream_id = ? AND registration_id = ?`, streamExpiry(), publicationID, streamID, registrationID)
			if err != nil {
				continue
			}
			if changed, err := result.RowsAffected(); err == nil && changed == 0 {
				return
			}
		}
	}
}

func streamExpiry() string {
	return time.Now().UTC().Add(streamLease).Format(time.RFC3339Nano)
}

func streamRegistrationID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
