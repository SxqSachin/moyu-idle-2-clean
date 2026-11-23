package riskinsight

import "time"

const (
	CategoryAbnormalTrade  = "abnormal_trade"
	CategoryLinkedAccounts = "linked_accounts"
	CategoryBanned         = "banned"
	CategoryAll            = "all"
)

type PublicUser struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
}

type PublicNoticeboardEntry struct {
	Category        string       `json:"category"`
	Users           []PublicUser `json:"users"`
	Status          string       `json:"status"`
	ConfidenceBand  string       `json:"confidenceBand"`
	SummaryText     string       `json:"summaryText"`
	WindowStartedAt time.Time    `json:"windowStartedAt"`
	WindowEndedAt   time.Time    `json:"windowEndedAt"`
	GeneratedAt     time.Time    `json:"generatedAt"`
	ExpiresAt       time.Time    `json:"expiresAt"`
	Stale           bool         `json:"stale"`
}

type PublicNoticeboardSnapshot struct {
	Category        string                   `json:"category"`
	Items           []PublicNoticeboardEntry `json:"items"`
	WindowStartedAt time.Time                `json:"windowStartedAt"`
	WindowEndedAt   time.Time                `json:"windowEndedAt"`
	GeneratedAt     time.Time                `json:"generatedAt"`
	ExpiresAt       time.Time                `json:"expiresAt"`
	Stale           bool                     `json:"stale"`
}
