package store

import (
	"context"
	"strings"
)

const defaultChatChannel = "world"

func normalizeChatChannel(channel string) string {
	trimmed := strings.TrimSpace(strings.ToLower(channel))
	if trimmed == "" {
		return defaultChatChannel
	}
	return trimmed
}

func clampChatHistoryLimit(limit int) int {
	if limit <= 0 {
		return 30
	}
	if limit > 200 {
		return 200
	}
	return limit
}

// ChatCosmeticMeta holds the cosmetic display data for a chat message.
type ChatCosmeticMeta struct {
	TitlePrefixID string
	TitleSuffixID string
	DisplayMeta   map[string]any // future: avatarId, avatarFrameId, etc.
	Attachment    map[string]any
}

func (s *Store) SaveChat(ctx context.Context, userID int64, content string) error {
	_, err := s.SaveChatMessage(ctx, userID, defaultChatChannel, content, ChatCosmeticMeta{})
	return err
}

func (s *Store) SaveChatMessage(ctx context.Context, userID int64, channel string, content string, meta ChatCosmeticMeta) (ChatMessage, error) {
	channel = normalizeChatChannel(channel)
	displayMeta := meta.DisplayMeta
	if displayMeta == nil {
		displayMeta = map[string]any{}
	}
	attachment := meta.Attachment
	if attachment == nil {
		attachment = map[string]any{}
	}
	var out ChatMessage
	err := s.db.QueryRow(ctx, `
		INSERT INTO chat_messages(channel, user_id, username, title_prefix_id, title_suffix_id, display_meta, attachment_jsonb, content, sent_at)
		SELECT $2, u.id, u.username, $4, $5, $6, $7, $3, now()
		FROM users u
		WHERE u.id = $1
		RETURNING id, channel, user_id, username, title_prefix_id, title_suffix_id, display_meta, attachment_jsonb, content, sent_at
	`, userID, channel, content, meta.TitlePrefixID, meta.TitleSuffixID, displayMeta, attachment).Scan(
		&out.ID, &out.Channel, &out.UserID, &out.Username,
		&out.TitlePrefixID, &out.TitleSuffixID, &out.DisplayMeta, &out.Attachment,
		&out.Content, &out.SentAt,
	)
	return out, err
}

func (s *Store) RecentChat(ctx context.Context, limit int32) ([]ChatMessage, error) {
	return s.ListChatMessages(ctx, defaultChatChannel, int(limit), 0)
}

func (s *Store) ListChatMessages(ctx context.Context, channel string, limit int, beforeID int64) ([]ChatMessage, error) {
	channel = normalizeChatChannel(channel)
	limit = clampChatHistoryLimit(limit)
	rows, err := s.db.Query(ctx, `
		SELECT id, channel, user_id, username, title_prefix_id, title_suffix_id, display_meta, attachment_jsonb, content, sent_at
		FROM chat_messages
		WHERE channel = $1
		  AND ($2::bigint <= 0 OR id < $2)
		ORDER BY id DESC
		LIMIT $3
	`, channel, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ChatMessage, 0, limit)
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(
			&m.ID, &m.Channel, &m.UserID, &m.Username,
			&m.TitlePrefixID, &m.TitleSuffixID, &m.DisplayMeta, &m.Attachment,
			&m.Content, &m.SentAt,
		); err != nil {
			return nil, err
		}
		if m.DisplayMeta == nil {
			m.DisplayMeta = map[string]any{}
		}
		if m.Attachment == nil {
			m.Attachment = map[string]any{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
