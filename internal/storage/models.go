package storage

import "time"

const (
	pendingActionStatusPending  = "pending"
	pendingActionStatusConsumed = "consumed"
	pendingActionStatusExpired  = "expired"
)

type AdminSession struct {
	ID           uint  `gorm:"primaryKey"`
	TelegramUser int64 `gorm:"uniqueIndex;not null"`
	AdminID      uint  `gorm:"index;not null"`
	Username     string
	JWTToken     string
	JWTExpiresAt time.Time
	RolesJSON    string
	PoliciesJSON string
	LastLoginAt  time.Time
	LastUsedAt   time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (AdminSession) TableName() string {
	return "admin_sessions"
}

type PendingAction struct {
	ID           uint   `gorm:"primaryKey"`
	TelegramUser int64  `gorm:"index;not null"`
	Action       string `gorm:"index;not null"`
	PayloadJSON  string
	ExpiresAt    time.Time  `gorm:"index;not null"`
	Status       string     `gorm:"index;not null"`
	ConsumedAt   *time.Time `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (PendingAction) TableName() string {
	return "pending_actions"
}

type ActionLog struct {
	ID           uint   `gorm:"primaryKey"`
	TelegramUser int64  `gorm:"index;not null"`
	AdminID      uint   `gorm:"index;not null"`
	Action       string `gorm:"index;not null"`
	Target       string
	MetadataJSON string
	Result       string
	CreatedAt    time.Time `gorm:"index"`
}

func (ActionLog) TableName() string {
	return "action_logs"
}
