package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sessiondb "github.com/Yacobolo/leapview/internal/dashboard/internal/db"
	"github.com/Yacobolo/leapview/internal/dashboard/session"
)

type Store struct {
	db    *sql.DB
	q     *sessiondb.Queries
	ttl   time.Duration
	clock func() time.Time
}

func NewStore(db *sql.DB) *Store {
	return NewStoreWithTTL(db, 5*time.Minute)
}

func NewStoreWithTTL(db *sql.DB, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Store{db: db, q: sessiondb.New(db), ttl: ttl, clock: time.Now}
}

func (store *Store) Create(ctx context.Context, key session.Key, state session.State) (session.Record, error) {
	if store.db == nil {
		return session.Record{}, fmt.Errorf("dashboard session database is required")
	}
	if err := key.Validate(); err != nil {
		return session.Record{}, err
	}
	keyJSON, stateJSON, err := encode(key, state)
	if err != nil {
		return session.Record{}, err
	}
	record := session.Record{Key: key, Version: 1, State: state, ExpiresAt: store.expiry()}
	err = store.q.CreateDashboardViewSession(ctx, sessiondb.CreateDashboardViewSessionParams{
		ID:        key.ID(),
		KeyJson:   keyJSON,
		StateJson: stateJSON,
		ExpiresAt: record.ExpiresAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		if _, loadErr := store.Load(ctx, key); loadErr == nil {
			return session.Record{}, session.ErrConflict
		}
		return session.Record{}, err
	}
	return record, nil
}

func (store *Store) Load(ctx context.Context, key session.Key) (session.Record, error) {
	row, err := store.q.GetActiveDashboardViewSession(ctx, sessiondb.GetActiveDashboardViewSessionParams{
		ID:  key.ID(),
		Now: store.clock().UTC().Format(time.RFC3339Nano),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return session.Record{}, session.ErrNotFound
	}
	if err != nil {
		return session.Record{}, err
	}
	var storedKey session.Key
	var state session.State
	if err := json.Unmarshal([]byte(row.KeyJson), &storedKey); err != nil {
		return session.Record{}, fmt.Errorf("decode dashboard session key: %w", err)
	}
	if storedKey != key {
		return session.Record{}, fmt.Errorf("dashboard session key digest collision")
	}
	if err := json.Unmarshal([]byte(row.StateJson), &state); err != nil {
		return session.Record{}, fmt.Errorf("decode dashboard session state: %w", err)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, row.ExpiresAt)
	if err != nil {
		return session.Record{}, fmt.Errorf("decode dashboard session expiry: %w", err)
	}
	return session.Record{Key: storedKey, Version: uint64(row.Version), State: state, ExpiresAt: expiresAt}, nil
}

func (store *Store) CompareAndSwap(ctx context.Context, key session.Key, version uint64, state session.State) (session.Record, error) {
	_, stateJSON, err := encode(key, state)
	if err != nil {
		return session.Record{}, err
	}
	expiresAt := store.expiry()
	result, err := store.q.CompareAndSwapDashboardViewSession(ctx, sessiondb.CompareAndSwapDashboardViewSessionParams{
		StateJson: stateJSON,
		ExpiresAt: expiresAt.Format(time.RFC3339Nano),
		ID:        key.ID(),
		Version:   int64(version),
		Now:       store.clock().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return session.Record{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return session.Record{}, err
	}
	if changed != 1 {
		if _, loadErr := store.Load(ctx, key); loadErr == nil {
			return session.Record{}, session.ErrConflict
		}
		return session.Record{}, session.ErrNotFound
	}
	return session.Record{Key: key, Version: version + 1, State: state, ExpiresAt: expiresAt}, nil
}

func (store *Store) Touch(ctx context.Context, key session.Key) error {
	result, err := store.q.TouchDashboardViewSession(ctx, sessiondb.TouchDashboardViewSessionParams{
		ExpiresAt: store.expiry().Format(time.RFC3339Nano),
		ID:        key.ID(),
		Now:       store.clock().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return session.ErrNotFound
	}
	return nil
}

func (store *Store) DeleteExpired(ctx context.Context) error {
	return store.q.DeleteExpiredDashboardViewSessions(ctx, store.clock().UTC().Format(time.RFC3339Nano))
}

func (store *Store) expiry() time.Time {
	return store.clock().UTC().Add(store.ttl)
}

func encode(key session.Key, state session.State) (string, string, error) {
	keyJSON, err := json.Marshal(key)
	if err != nil {
		return "", "", err
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return "", "", err
	}
	return string(keyJSON), string(stateJSON), nil
}
