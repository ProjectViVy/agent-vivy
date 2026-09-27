package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func (b *Backend) EnqueueChildMessage(ctx context.Context, message domain.ChildMailboxMessage) (domain.ChildMailboxMessage, bool, error) {
	if err := validateChildMailboxMessage(message); err != nil {
		return message, false, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return message, false, fmt.Errorf("storage: begin child message admission: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	binding, err := sqliteReadChildBinding(ctx, tx, message.ChildSessionID)
	if err != nil {
		return message, false, err
	}
	if err := sqliteLockAdmissionSession(ctx, tx, binding.OriginParentSessionID); err != nil {
		return message, false, err
	}
	if err := sqliteLockChildBinding(ctx, tx, message.ChildSessionID); err != nil {
		return message, false, err
	}
	binding, err = sqliteReadChildBinding(ctx, tx, message.ChildSessionID)
	if err != nil {
		return message, false, err
	}
	if binding.State != domain.ChildSessionOpen {
		return message, false, storage.ErrChildSessionClosed
	}
	if !storage.ValidDirectChildMailboxRoute(binding, message.SenderSessionID, message.RecipientSessionID) {
		return message, false, storage.ErrChildMessageConflict
	}
	if existing, found, err := sqliteReadChildMessageByKey(ctx, tx, message.ChildSessionID, message.SenderSessionID, message.IdempotencyKey); err != nil {
		return message, false, err
	} else if found {
		if string(existing.Body) != string(message.Body) {
			return message, false, storage.ErrChildMessageConflict
		}
		if err := tx.Commit(); err != nil {
			return message, false, fmt.Errorf("storage: commit idempotent child message: %w", err)
		}
		return existing, false, nil
	}
	if message.Sequence != 0 {
		return message, false, storage.ErrChildMessageConflict
	}
	if message.CreatedAt <= 0 {
		message.CreatedAt = time.Now().UnixMilli()
	}
	nextSequence := binding.NextMessageSequence
	sequenceColumn := "next_message_sequence"
	if message.RecipientSessionID == binding.OriginParentSessionID {
		nextSequence = binding.NextParentMessageSequence
		sequenceColumn = "next_parent_message_sequence"
	}
	if nextSequence > storage.MaxChildMailboxMessagesPerRecipient {
		return message, false, storage.ErrChildMailboxFull
	}
	message.Sequence = nextSequence
	message.Status = domain.ChildMessagePending
	if _, err := tx.ExecContext(ctx, `UPDATE child_sessions SET `+sequenceColumn+` = ?, updated_at = ? WHERE child_session_id = ? AND state = 'open' AND `+sequenceColumn+` = ?`,
		message.Sequence+1, message.CreatedAt, message.ChildSessionID, message.Sequence); err != nil {
		return message, false, fmt.Errorf("storage: advance child mailbox sequence: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO child_mailbox_messages
			(message_id,child_session_id,sender_session_id,recipient_session_id,idempotency_key,sequence,body,status,created_at,consumed_at,consumed_by_run_id)
		VALUES (?,?,?,?,?,?,?,?,?,0,'')`, message.ID, message.ChildSessionID, message.SenderSessionID, message.RecipientSessionID,
		message.IdempotencyKey, message.Sequence, message.Body, string(message.Status), message.CreatedAt); err != nil {
		return message, false, storage.AdmissionUnavailable("insert child mailbox message", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END WHERE id IN (?,?)`,
		message.CreatedAt, message.CreatedAt, message.ChildSessionID, binding.OriginParentSessionID); err != nil {
		return message, false, fmt.Errorf("storage: touch child mailbox sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return message, false, storage.AdmissionUnavailable("commit child mailbox message", err)
	}
	message.Body = append([]byte(nil), message.Body...)
	return message, true, nil
}

func (b *Backend) ListPendingChildMessages(ctx context.Context, childSessionID, recipientSessionID domain.SessionID, afterSequence int64, limit int) ([]domain.ChildMailboxMessage, error) {
	if childSessionID == "" || recipientSessionID == "" || afterSequence < 0 {
		return nil, errors.New("storage: invalid child mailbox cursor")
	}
	binding, err := b.GetChildSessionBinding(ctx, childSessionID)
	if err != nil {
		return nil, err
	}
	if recipientSessionID != childSessionID && recipientSessionID != binding.OriginParentSessionID {
		return nil, storage.ErrChildMessageConflict
	}
	if limit <= 0 {
		return []domain.ChildMailboxMessage{}, nil
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := b.db.QueryContext(ctx, `
		SELECT message_id,child_session_id,sender_session_id,recipient_session_id,idempotency_key,sequence,body,status,created_at,consumed_at,consumed_by_run_id
		FROM child_mailbox_messages WHERE child_session_id = ? AND recipient_session_id = ? AND status = 'pending' AND sequence > ?
		ORDER BY sequence LIMIT ?`, childSessionID, recipientSessionID, afterSequence, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list pending child messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.ChildMailboxMessage, 0)
	for rows.Next() {
		var item domain.ChildMailboxMessage
		var childID, senderID, recipientID, status, consumedRunID string
		if err := rows.Scan(&item.ID, &childID, &senderID, &recipientID, &item.IdempotencyKey, &item.Sequence, &item.Body,
			&status, &item.CreatedAt, &item.ConsumedAt, &consumedRunID); err != nil {
			return nil, fmt.Errorf("storage: scan pending child message: %w", err)
		}
		item.ChildSessionID, item.SenderSessionID, item.RecipientSessionID = domain.SessionID(childID), domain.SessionID(senderID), domain.SessionID(recipientID)
		item.Status, item.ConsumedByRunID = domain.ChildMessageStatus(status), domain.RunID(consumedRunID)
		item.Body = append([]byte(nil), item.Body...)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (b *Backend) RecordChildMessageReceipt(ctx context.Context, receipt domain.ChildMessageReceipt) (domain.ChildMessageReceipt, bool, error) {
	if err := validateChildMessageReceipt(receipt); err != nil {
		return receipt, false, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return receipt, false, fmt.Errorf("storage: begin child message receipt: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	binding, err := sqliteReadChildBinding(ctx, tx, receipt.ChildSessionID)
	if err != nil {
		return receipt, false, err
	}
	if err := sqliteLockAdmissionSession(ctx, tx, binding.OriginParentSessionID); err != nil {
		return receipt, false, err
	}
	if err := sqliteLockChildBinding(ctx, tx, receipt.ChildSessionID); err != nil {
		return receipt, false, err
	}
	binding, err = sqliteReadChildBinding(ctx, tx, receipt.ChildSessionID)
	if err != nil {
		return receipt, false, err
	}
	message, found, err := sqliteReadChildMailboxMessage(ctx, tx, receipt.ChildSessionID, receipt.MessageID)
	if err != nil {
		return receipt, false, err
	}
	if !found {
		return receipt, false, storage.ErrNotFound
	}
	if !storage.ValidDirectChildMailboxRoute(binding, message.SenderSessionID, message.RecipientSessionID) {
		return receipt, false, storage.ErrChildMessageConflict
	}
	if stored, found, err := sqliteReadChildMessageReceipt(ctx, tx, receipt.ChildSessionID, receipt.MessageID, receipt.ConsumerRunID); err != nil {
		return receipt, false, err
	} else if found {
		if stored.State == receipt.State {
			if err := tx.Commit(); err != nil {
				return receipt, false, fmt.Errorf("storage: commit repeated child receipt: %w", err)
			}
			return stored, false, nil
		}
		if stored.State != domain.ChildMessageReceiptInProgress || receipt.State == domain.ChildMessageReceiptInProgress {
			return receipt, false, storage.ErrChildMessageConflict
		}
		receipt.CreatedAt = stored.CreatedAt
		if receipt.UpdatedAt <= 0 {
			receipt.UpdatedAt = time.Now().UnixMilli()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE child_message_receipts SET state = ?, updated_at = ? WHERE child_session_id = ? AND message_id = ? AND consumer_run_id = ? AND state = 'in-progress'`,
			string(receipt.State), receipt.UpdatedAt, receipt.ChildSessionID, receipt.MessageID, receipt.ConsumerRunID); err != nil {
			return receipt, false, fmt.Errorf("storage: settle child receipt: %w", err)
		}
	} else {
		if binding.State != domain.ChildSessionOpen || message.Status != domain.ChildMessagePending {
			return receipt, false, storage.ErrChildSessionClosed
		}
		if err := sqliteValidateActiveChildMessageConsumer(ctx, tx, binding, message.RecipientSessionID, receipt.ConsumerRunID); err != nil {
			return receipt, false, err
		}
		if receipt.CreatedAt <= 0 {
			receipt.CreatedAt = time.Now().UnixMilli()
		}
		if receipt.UpdatedAt <= 0 {
			receipt.UpdatedAt = receipt.CreatedAt
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO child_message_receipts (child_session_id,message_id,consumer_run_id,state,created_at,updated_at) VALUES (?,?,?,?,?,?)`,
			receipt.ChildSessionID, receipt.MessageID, receipt.ConsumerRunID, string(receipt.State), receipt.CreatedAt, receipt.UpdatedAt); err != nil {
			return receipt, false, storage.AdmissionUnavailable("insert child message receipt", err)
		}
	}
	if receipt.State == domain.ChildMessageReceiptConsumed {
		cursor, cursorColumn := binding.ConsumedMessageSequence, "consumed_message_sequence"
		if message.RecipientSessionID == binding.OriginParentSessionID {
			cursor, cursorColumn = binding.ConsumedParentMessageSequence, "consumed_parent_message_sequence"
		}
		if message.Status != domain.ChildMessagePending || message.Sequence != cursor+1 || binding.State != domain.ChildSessionOpen {
			return receipt, false, storage.ErrChildMessageConflict
		}
		if err := sqliteValidateActiveChildMessageConsumer(ctx, tx, binding, message.RecipientSessionID, receipt.ConsumerRunID); err != nil {
			return receipt, false, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE child_mailbox_messages SET status = 'consumed',consumed_at = ?,consumed_by_run_id = ? WHERE child_session_id = ? AND message_id = ? AND status = 'pending'`,
			receipt.UpdatedAt, receipt.ConsumerRunID, receipt.ChildSessionID, receipt.MessageID); err != nil {
			return receipt, false, fmt.Errorf("storage: consume child mailbox message: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE child_sessions SET `+cursorColumn+` = ?,updated_at = ? WHERE child_session_id = ? AND `+cursorColumn+` = ?`,
			message.Sequence, receipt.UpdatedAt, receipt.ChildSessionID, cursor); err != nil {
			return receipt, false, fmt.Errorf("storage: advance child message cursor: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return receipt, false, storage.AdmissionUnavailable("commit child message receipt", err)
	}
	return receipt, true, nil
}

func (b *Backend) GetChildMessageReceipt(ctx context.Context, childSessionID domain.SessionID, messageID string, consumerRunID domain.RunID) (domain.ChildMessageReceipt, error) {
	receipt, found, err := sqliteReadChildMessageReceipt(ctx, b.db, childSessionID, messageID, consumerRunID)
	if err != nil {
		return domain.ChildMessageReceipt{}, err
	}
	if !found {
		return domain.ChildMessageReceipt{}, storage.ErrNotFound
	}
	return receipt, nil
}

func (b *Backend) CloseChildSession(ctx context.Context, childSessionID domain.SessionID, at int64) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin close child session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	binding, err := sqliteReadChildBinding(ctx, tx, childSessionID)
	if err != nil {
		return err
	}
	if err := sqliteLockAdmissionSession(ctx, tx, binding.OriginParentSessionID); err != nil {
		return err
	}
	if err := sqliteLockChildBinding(ctx, tx, childSessionID); err != nil {
		return err
	}
	if at <= 0 {
		at = time.Now().UnixMilli()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE child_sessions SET state = 'closed',updated_at = ? WHERE child_session_id = ? AND state = 'open'`, at, childSessionID); err != nil {
		return fmt.Errorf("storage: close child session: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE child_mailbox_messages SET status = 'expired' WHERE child_session_id = ? AND status = 'pending'`, childSessionID); err != nil {
		return fmt.Errorf("storage: expire child messages: %w", err)
	}
	return tx.Commit()
}

func sqliteLockChildBinding(ctx context.Context, tx *sql.Tx, childID domain.SessionID) error {
	result, err := tx.ExecContext(ctx, `UPDATE child_sessions SET updated_at = updated_at WHERE child_session_id = ?`, childID)
	if err != nil {
		return fmt.Errorf("storage: lock child session binding: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: verify child session lock: %w", err)
	}
	if count == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func validateChildMailboxMessage(message domain.ChildMailboxMessage) error {
	if message.ID == "" || len(message.ID) > 128 || message.ChildSessionID == "" || message.SenderSessionID == "" || message.RecipientSessionID == "" || message.IdempotencyKey == "" || len(message.IdempotencyKey) > 256 || len(message.Body) == 0 || len(message.Body) > storage.MaxChildMessageBytes {
		return errors.New("storage: child message identity and body are required")
	}
	if (message.Status != "" && message.Status != domain.ChildMessagePending) || message.ConsumedAt != 0 || message.ConsumedByRunID != "" {
		return errors.New("storage: child message must be newly pending")
	}
	return nil
}

func validateChildMessageReceipt(receipt domain.ChildMessageReceipt) error {
	if receipt.ChildSessionID == "" || receipt.MessageID == "" || receipt.ConsumerRunID == "" || !receipt.State.Valid() {
		return errors.New("storage: child message receipt identity or state is invalid")
	}
	return nil
}

func sqliteReadChildMessageByKey(ctx context.Context, tx *sql.Tx, childID, senderID domain.SessionID, key string) (domain.ChildMailboxMessage, bool, error) {
	var message domain.ChildMailboxMessage
	var child, sender, recipient, status, consumedRun string
	err := tx.QueryRowContext(ctx, `SELECT message_id,child_session_id,sender_session_id,recipient_session_id,idempotency_key,sequence,body,status,created_at,consumed_at,consumed_by_run_id FROM child_mailbox_messages WHERE child_session_id = ? AND sender_session_id = ? AND idempotency_key = ?`, childID, senderID, key).Scan(
		&message.ID, &child, &sender, &recipient, &message.IdempotencyKey, &message.Sequence, &message.Body, &status, &message.CreatedAt, &message.ConsumedAt, &consumedRun)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ChildMailboxMessage{}, false, nil
	}
	if err != nil {
		return domain.ChildMailboxMessage{}, false, fmt.Errorf("storage: read idempotent child message: %w", err)
	}
	message.ChildSessionID, message.SenderSessionID, message.RecipientSessionID = domain.SessionID(child), domain.SessionID(sender), domain.SessionID(recipient)
	message.Status, message.ConsumedByRunID = domain.ChildMessageStatus(status), domain.RunID(consumedRun)
	message.Body = append([]byte(nil), message.Body...)
	return message, true, nil
}

func sqliteReadChildMessageReceipt(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, childID domain.SessionID, messageID string, runID domain.RunID) (domain.ChildMessageReceipt, bool, error) {
	var receipt domain.ChildMessageReceipt
	err := q.QueryRowContext(ctx, `SELECT child_session_id,message_id,consumer_run_id,state,created_at,updated_at FROM child_message_receipts WHERE child_session_id = ? AND message_id = ? AND consumer_run_id = ?`, childID, messageID, runID).Scan(
		(*string)(&receipt.ChildSessionID), &receipt.MessageID, (*string)(&receipt.ConsumerRunID), (*string)(&receipt.State), &receipt.CreatedAt, &receipt.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ChildMessageReceipt{}, false, nil
	}
	if err != nil {
		return domain.ChildMessageReceipt{}, false, fmt.Errorf("storage: read child message receipt: %w", err)
	}
	return receipt, true, nil
}

func sqliteReadChildMailboxMessage(ctx context.Context, tx *sql.Tx, childID domain.SessionID, messageID string) (domain.ChildMailboxMessage, bool, error) {
	var message domain.ChildMailboxMessage
	var childIDValue, senderID, recipientID, status, consumedRun string
	err := tx.QueryRowContext(ctx, `SELECT message_id,child_session_id,sender_session_id,recipient_session_id,idempotency_key,sequence,body,status,created_at,consumed_at,consumed_by_run_id FROM child_mailbox_messages WHERE child_session_id = ? AND message_id = ?`, childID, messageID).Scan(
		&message.ID, &childIDValue, &senderID, &recipientID, &message.IdempotencyKey, &message.Sequence, &message.Body,
		&status, &message.CreatedAt, &message.ConsumedAt, &consumedRun)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ChildMailboxMessage{}, false, nil
	}
	if err != nil {
		return domain.ChildMailboxMessage{}, false, fmt.Errorf("storage: read child mailbox message: %w", err)
	}
	message.ChildSessionID, message.SenderSessionID, message.RecipientSessionID = domain.SessionID(childIDValue), domain.SessionID(senderID), domain.SessionID(recipientID)
	message.Status, message.ConsumedByRunID = domain.ChildMessageStatus(status), domain.RunID(consumedRun)
	message.Body = append([]byte(nil), message.Body...)
	return message, true, nil
}

func sqliteValidateActiveChildMessageConsumer(ctx context.Context, tx *sql.Tx, binding domain.ChildSessionBinding, recipientID domain.SessionID, runID domain.RunID) error {
	var sessionID, status, kind, mode string
	err := tx.QueryRowContext(ctx, `SELECT session_id,status,kind,child_mode FROM runs WHERE id = ?`, runID).Scan(&sessionID, &status, &kind, &mode)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("storage: validate child message consumer: %w", err)
	}
	if status != string(domain.RunActive) {
		return storage.ErrChildAdmissionConflict
	}
	if recipientID == binding.ChildSessionID {
		if runID != binding.ActivationRunID || sessionID != string(binding.ChildSessionID) || kind != string(domain.RunKindChild) || mode != string(domain.ChildModeContinuable) {
			return storage.ErrChildAdmissionConflict
		}
		return nil
	}
	if recipientID != binding.OriginParentSessionID || sessionID != string(binding.OriginParentSessionID) {
		return storage.ErrChildAdmissionConflict
	}
	return nil
}
