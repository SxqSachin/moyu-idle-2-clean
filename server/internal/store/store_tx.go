package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (s *Store) lockUserTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	if s.transactionHasUserLock(tx, userID) {
		return nil
	}
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, userID)
	if err == nil {
		s.recordTransactionUserLock(tx, userID)
	}
	return err
}

func (s *Store) tryLockUserTx(ctx context.Context, tx pgx.Tx, userID int64) (bool, error) {
	if userID <= 0 {
		return false, errors.New("invalid user id")
	}
	if s.transactionHasUserLock(tx, userID) {
		return true, nil
	}
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, userID).Scan(&locked); err != nil {
		return false, err
	}
	if locked {
		s.recordTransactionUserLock(tx, userID)
	}
	return locked, nil
}

func (s *Store) transactionHasUserLock(tx pgx.Tx, userID int64) bool {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	buffer := s.auditBuffers[tx]
	if buffer == nil {
		return false
	}
	_, ok := buffer.lockedUserIDs[userID]
	return ok
}

func (s *Store) recordTransactionUserLock(tx pgx.Tx, userID int64) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	buffer := s.auditBuffers[tx]
	if buffer == nil {
		return
	}
	if buffer.lockedUserIDs == nil {
		buffer.lockedUserIDs = make(map[int64]struct{})
	}
	buffer.lockedUserIDs[userID] = struct{}{}
}

func (s *Store) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	s.registerInventoryAuditBuffer(tx)
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		s.discardInventoryAuditBuffer(tx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		s.discardInventoryAuditBuffer(tx)
		return err
	}
	s.flushInventoryAuditBuffer(ctx, tx)
	return nil
}

func (s *Store) withReadOnlyTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return s.withTx(ctx, fn)
}

func (s *Store) Pool() *pgxpool.Pool {
	return s.db
}

func (s *Store) ItemEconomyBaseValue(itemID string) int64 {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return 0
	}
	for _, item := range s.gameData.Items {
		if strings.TrimSpace(item.ID) != itemID {
			continue
		}
		if item.EconomyBaseValue <= 0 {
			return 0
		}
		return int64(item.EconomyBaseValue)
	}
	return 0
}
