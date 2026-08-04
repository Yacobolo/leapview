package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/flidai/leapview/internal/access/desktopauth"
	accessdb "github.com/flidai/leapview/internal/access/internal/db"
)

func (r *Repository) StoreAuthorizationCode(
	ctx context.Context,
	code desktopauth.AuthorizationCode,
) error {
	if r == nil || r.q == nil {
		return fmt.Errorf("desktop authorization database is required")
	}
	if err := r.q.CleanDesktopAuthorizationCodes(
		ctx,
		code.CreatedAt.Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("clean desktop authorization codes: %w", err)
	}
	if err := r.q.CreateDesktopAuthorizationCode(
		ctx,
		accessdb.CreateDesktopAuthorizationCodeParams{
			CodeHash:      code.CodeHash[:],
			PrincipalID:   code.PrincipalID,
			ClientID:      code.ClientID,
			InstanceID:    code.InstanceID,
			ProfileID:     code.ProfileID,
			RedirectUri:   code.RedirectURI,
			CodeChallenge: code.CodeChallenge,
			ReturnPath:    code.ReturnPath,
			ExpiresAt:     code.ExpiresAt.Format(time.RFC3339Nano),
			CreatedAt:     code.CreatedAt.Format(time.RFC3339Nano),
		},
	); err != nil {
		return fmt.Errorf("create desktop authorization code: %w", err)
	}
	return nil
}

func (r *Repository) ConsumeAuthorizationCode(
	ctx context.Context,
	codeHash [32]byte,
	now time.Time,
	validate func(desktopauth.AuthorizationCode) bool,
) (string, error) {
	if r == nil || r.root == nil || r.q == nil {
		return "", fmt.Errorf("desktop authorization database is required")
	}
	if validate == nil {
		return "", fmt.Errorf("desktop authorization validator is required")
	}
	if _, transactional := r.db.(*sql.Tx); transactional {
		return consumeAuthorizationCode(ctx, r.q, codeHash, now, validate)
	}
	transaction, err := r.root.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin desktop authorization redemption: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	principalID, err := consumeAuthorizationCode(
		ctx,
		r.q.WithTx(transaction),
		codeHash,
		now,
		validate,
	)
	if err != nil {
		return "", err
	}
	if err := transaction.Commit(); err != nil {
		return "", fmt.Errorf("commit desktop authorization redemption: %w", err)
	}
	return principalID, nil
}

func consumeAuthorizationCode(
	ctx context.Context,
	queries *accessdb.Queries,
	codeHash [32]byte,
	now time.Time,
	validate func(desktopauth.AuthorizationCode) bool,
) (string, error) {
	grant, err := readAuthorizationCode(ctx, queries, codeHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", desktopauth.ErrInvalidGrant
	}
	if err != nil {
		return "", err
	}
	if !validate(grant) {
		return "", desktopauth.ErrInvalidGrant
	}
	affected, err := queries.ConsumeDesktopAuthorizationCode(
		ctx,
		accessdb.ConsumeDesktopAuthorizationCodeParams{
			ConsumedAt: sql.NullString{
				String: now.Format(time.RFC3339Nano),
				Valid:  true,
			},
			CodeHash: codeHash[:],
		},
	)
	if err != nil {
		return "", fmt.Errorf("consume desktop authorization code: %w", err)
	}
	if affected != 1 {
		return "", desktopauth.ErrInvalidGrant
	}
	return grant.PrincipalID, nil
}

func readAuthorizationCode(
	ctx context.Context,
	queries *accessdb.Queries,
	codeHash [32]byte,
) (desktopauth.AuthorizationCode, error) {
	row, err := queries.GetDesktopAuthorizationCode(ctx, codeHash[:])
	if err != nil {
		return desktopauth.AuthorizationCode{}, err
	}
	grant := desktopauth.AuthorizationCode{
		CodeHash:      codeHash,
		PrincipalID:   row.PrincipalID,
		ClientID:      row.ClientID,
		InstanceID:    row.InstanceID,
		ProfileID:     row.ProfileID,
		RedirectURI:   row.RedirectUri,
		CodeChallenge: row.CodeChallenge,
		ReturnPath:    row.ReturnPath,
		Consumed:      row.ConsumedAt.Valid,
	}
	grant.ExpiresAt, err = time.Parse(time.RFC3339Nano, row.ExpiresAt)
	if err != nil {
		return desktopauth.AuthorizationCode{}, fmt.Errorf(
			"parse desktop authorization expiry: %w",
			err,
		)
	}
	grant.CreatedAt, err = time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return desktopauth.AuthorizationCode{}, fmt.Errorf(
			"parse desktop authorization creation time: %w",
			err,
		)
	}
	return grant, nil
}
