package risk

import "time"

type DeviceProfile struct {
	FingerprintID      string  `json:"fingerprintId"`
	FingerprintVersion string  `json:"fingerprintVersion"`
	ComponentsHash     string  `json:"componentsHash"`
	DeviceFamilyHash   string  `json:"deviceFamilyHash"`
	BrowserFamilyHash  string  `json:"browserFamilyHash"`
	RuntimeFamilyHash  string  `json:"runtimeFamilyHash"`
	CookieID           string  `json:"cookieId"`
	LocalDeviceID      string  `json:"localDeviceId"`
	BrowserInstallID   string  `json:"browserInstallId"`
	SessionChainID     string  `json:"sessionChainId"`
	Timezone           string  `json:"timezone"`
	Language           string  `json:"language"`
	ScreenBucket       string  `json:"screenBucket"`
	CanvasHash         string  `json:"canvasHash"`
	WebGLHash          string  `json:"webglHash"`
	AudioHash          string  `json:"audioHash"`
	PlatformHint       string  `json:"platformHint"`
	CPUHint            string  `json:"cpuHint"`
	MemoryHint         string  `json:"memoryHint"`
	CaptchaScore       float64 `json:"captchaScore"`
}

type SignalInput struct {
	UserID         int64
	EventType      string
	IP             string
	UserAgent      string
	AcceptLanguage string
	CountryCode    string
	ASN            string
	PasswordRaw    string
	DeviceProfile  DeviceProfile
	Extra          map[string]any
}

type UserContext struct {
	UserID                 int64             `json:"userId"`
	ClusterID              *int64            `json:"clusterId,omitempty"`
	RiskScore              int               `json:"riskScore"`
	RiskLevel              string            `json:"riskLevel"`
	ClusterStatus          string            `json:"clusterStatus"`
	RestrictedCapabilities []CapabilityBlock `json:"restrictedCapabilities"`
}

type CapabilityBlock struct {
	Capability string     `json:"capability"`
	BlockLevel string     `json:"blockLevel"`
	ReasonCode string     `json:"reasonCode"`
	Source     string     `json:"source"`
	ClusterID  *int64     `json:"clusterId,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type UserSummary struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
}

type EvidenceRow struct {
	Peer         UserSummary    `json:"peer"`
	EvidenceType string         `json:"evidenceType"`
	Strength     string         `json:"strength"`
	Score        int            `json:"score"`
	FirstSeenAt  time.Time      `json:"firstSeenAt"`
	LastSeenAt   time.Time      `json:"lastSeenAt"`
	Payload      map[string]any `json:"payload"`
}

type TimelineRow struct {
	EventType         string         `json:"eventType"`
	OccurredAt        time.Time      `json:"occurredAt"`
	IP                string         `json:"ip"`
	IPPrefix          string         `json:"ipPrefix"`
	FingerprintID     string         `json:"fingerprintId"`
	LocalDeviceID     string         `json:"localDeviceId"`
	DeviceFamilyHash  string         `json:"deviceFamilyHash"`
	BrowserFamilyHash string         `json:"browserFamilyHash"`
	Timezone          string         `json:"timezone"`
	Language          string         `json:"language"`
	Extra             map[string]any `json:"extra"`
}

type UserDetailInfo struct {
	UserID    int64     `json:"userId"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

type UserDetail struct {
	User          UserDetailInfo        `json:"user"`
	Context       UserContext           `json:"context"`
	Signals       []TimelineRow         `json:"signals"`
	Transfers     []TransferRow         `json:"transfers"`
	TransferPeers []TransferPeerSummary `json:"transferPeers"`
	Evidences     []EvidenceRow         `json:"evidences"`
	Blocks        []CapabilityBlock     `json:"blocks"`
	LinkedUsers   []LinkedUser          `json:"linkedUsers"`
	ReviewCases   []ReviewCase          `json:"reviewCases"`
}

type LinkedUser struct {
	Peer        UserSummary `json:"peer"`
	TotalScore  int         `json:"totalScore"`
	RiskLevel   string      `json:"riskLevel"`
	FirstSeenAt time.Time   `json:"firstSeenAt"`
	LastSeenAt  time.Time   `json:"lastSeenAt"`
}

type ClusterDetail struct {
	ClusterID   int64           `json:"clusterId"`
	RiskScore   int             `json:"riskScore"`
	RiskLevel   string          `json:"riskLevel"`
	Status      string          `json:"status"`
	MemberCount int             `json:"memberCount"`
	PrimaryUser *UserSummary    `json:"primaryUser,omitempty"`
	Members     []ClusterMember `json:"members"`
	ReviewCases []ReviewCase    `json:"reviewCases"`
}

type ClusterMember struct {
	User          UserSummary `json:"user"`
	MemberScore   int         `json:"memberScore"`
	ReasonSummary string      `json:"reasonSummary"`
	JoinedAt      time.Time   `json:"joinedAt"`
	IsBanned      bool        `json:"isBanned"`
}

type ReviewCase struct {
	ID           int64     `json:"id"`
	ClusterID    *int64    `json:"clusterId,omitempty"`
	UserID       *int64    `json:"userId,omitempty"`
	Status       string    `json:"status"`
	Decision     string    `json:"decision"`
	OperatorName string    `json:"operatorName"`
	OperatorNote string    `json:"operatorNote"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type SearchResult struct {
	UserID        int64      `json:"userId"`
	Username      string     `json:"username"`
	ClusterID     *int64     `json:"clusterId,omitempty"`
	RiskLevel     string     `json:"riskLevel"`
	RiskScore     int        `json:"riskScore"`
	LocalDeviceID string     `json:"localDeviceId"`
	FingerprintID string     `json:"fingerprintId"`
	IP            string     `json:"ip"`
	OccurredAt    *time.Time `json:"occurredAt,omitempty"`
	IsBanned      bool       `json:"isBanned"`
}

type HighRiskUserItem struct {
	User          UserSummary `json:"user"`
	ClusterID     *int64      `json:"clusterId,omitempty"`
	RiskScore     int         `json:"riskScore"`
	RiskLevel     string      `json:"riskLevel"`
	ClusterStatus string      `json:"clusterStatus"`
	LastSeenAt    *time.Time  `json:"lastSeenAt,omitempty"`
	IsBanned      bool        `json:"isBanned"`
}

type PaginatedResult[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type ClusterSummary struct {
	ClusterID   int64        `json:"clusterId"`
	RiskScore   int          `json:"riskScore"`
	RiskLevel   string       `json:"riskLevel"`
	Status      string       `json:"status"`
	MemberCount int          `json:"memberCount"`
	PrimaryUser *UserSummary `json:"primaryUser,omitempty"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type TransferRow struct {
	Peer        UserSummary    `json:"peer"`
	Direction   string         `json:"direction"`
	Channel     string         `json:"channel"`
	ReferenceID int64          `json:"referenceId"`
	ItemID      string         `json:"itemId"`
	Quantity    int64          `json:"quantity"`
	UnitPrice   int64          `json:"unitPrice"`
	TotalPrice  int64          `json:"totalPrice"`
	FeeAmount   int64          `json:"feeAmount"`
	NetAmount   int64          `json:"netAmount"`
	PriceAnchor int64          `json:"priceAnchor"`
	PriceRatio  float64        `json:"priceRatio"`
	OccurredAt  time.Time      `json:"occurredAt"`
	Extra       map[string]any `json:"extra"`
}

type TransferPeerSummary struct {
	Peer              UserSummary `json:"peer"`
	TradeCount        int         `json:"tradeCount"`
	MarketCount       int         `json:"marketCount"`
	AuctionCount      int         `json:"auctionCount"`
	OutgoingCount     int         `json:"outgoingCount"`
	IncomingCount     int         `json:"incomingCount"`
	OutgoingNetAmount int64       `json:"outgoingNetAmount"`
	IncomingNetAmount int64       `json:"incomingNetAmount"`
	MaxSingleNet      int64       `json:"maxSingleNet"`
	LastOccurredAt    time.Time   `json:"lastOccurredAt"`
}

type DispositionRequest struct {
	ClusterID    int64
	Action       string
	OperatorName string
	OperatorNote string
}

type PasswordGroupMember struct {
	UserID    int64  `json:"userId"`
	Username  string `json:"username"`
	RiskLevel string `json:"riskLevel"`
	RiskScore int    `json:"riskScore"`
	IsBanned  bool   `json:"isBanned"`
}

type AuctionTransferItem struct {
	ID          int64          `json:"id"`
	FromUser    UserSummary    `json:"fromUser"`
	ToUser      UserSummary    `json:"toUser"`
	Channel     string         `json:"channel"`
	ReferenceID int64          `json:"referenceId"`
	ItemID      string         `json:"itemId"`
	Quantity    int64          `json:"quantity"`
	UnitPrice   int64          `json:"unitPrice"`
	TotalPrice  int64          `json:"totalPrice"`
	FeeAmount   int64          `json:"feeAmount"`
	NetAmount   int64          `json:"netAmount"`
	PriceAnchor int64          `json:"priceAnchor"`
	PriceRatio  float64        `json:"priceRatio"`
	OccurredAt  time.Time      `json:"occurredAt"`
	Extra       map[string]any `json:"extra"`
}

type PasswordGroup struct {
	HashDigestHint string                `json:"hashDigestHint"`
	Users          []PasswordGroupMember `json:"users"`
}
