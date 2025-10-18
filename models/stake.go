package models

import (
	"time"
	"github.com/google/uuid"
)

// Participant represents participant record (matches new database schema)
type Participant struct {
	ID        uuid.UUID `json:"id" db:"id"`
	EventID   int64     `json:"event_id" db:"event_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	IsAttend  bool      `json:"is_attend" db:"is_attend"`
	IsClaim   bool      `json:"is_claim" db:"is_claim"`
}

// CreateParticipantRequest for creating a new participant
type CreateParticipantRequest struct {
	EventID int64     `json:"event_id" binding:"required"`
	UserID  uuid.UUID `json:"user_id" binding:"required"`
}

// Stake represents a participant's stake for an event
type Stake struct {
	ID                    uuid.UUID `json:"id" db:"id"`
	EventID               int64     `json:"event_id" db:"event_id"`
	UserID                uuid.UUID `json:"user_id" db:"user_id"`
	WalletAddress         string    `json:"wallet_address" db:"wallet_address"`
	IsAttended            bool      `json:"is_attended" db:"is_attended"`
	StakeAmount           string    `json:"stake_amount" db:"stake_amount"`
	StakeTransactionHash  string    `json:"stake_transaction_hash" db:"stake_transaction_hash"`
	CreatedAtBlock        int64     `json:"created_at_block" db:"created_at_block"`
	CreatedAtTimestamp    time.Time `json:"created_at_timestamp" db:"created_at_timestamp"`
	RewardAmount          string    `json:"reward_amount" db:"reward_amount"`
	Claimed               bool      `json:"claimed" db:"claimed"`
	ClaimedTransactionHash string   `json:"claimed_transaction_hash" db:"claimed_transaction_hash"`
	ClaimedAt             *time.Time `json:"claimed_at" db:"claimed_at"`
	CreatedAt             time.Time `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`
}

// CreateStakeRequest for creating a new stake
type CreateStakeRequest struct {
	EventID               int64     `json:"event_id" binding:"required"`
	UserID                uuid.UUID `json:"user_id" binding:"required"`
	WalletAddress         string    `json:"wallet_address" binding:"required"`
	StakeAmount           string    `json:"stake_amount" binding:"required"`
	StakeTransactionHash  string    `json:"stake_transaction_hash" binding:"required"`
	CreatedAtBlock        int64     `json:"created_at_block" binding:"required"`
	CreatedAtTimestamp    time.Time `json:"created_at_timestamp" binding:"required"`
}

// UpdateStakeRequest for updating stake information
type UpdateStakeRequest struct {
	IsAttended            bool      `json:"is_attended"`
	RewardAmount          string    `json:"reward_amount"`
	Claimed               bool      `json:"claimed"`
	ClaimedTransactionHash string   `json:"claimed_transaction_hash"`
}

// Checkin represents QR code check-in records
type Checkin struct {
	ID           uuid.UUID `json:"id" db:"id"`
	EventID      int64     `json:"event_id" db:"event_id"`
	UserID       uuid.UUID `json:"user_id" db:"user_id"`
	WalletAddress string   `json:"wallet_address" db:"wallet_address"`
	QRCodeData   string    `json:"qr_code_data" db:"qr_code_data"`
	CheckinTime  time.Time `json:"checkin_time" db:"checkin_time"`
	CheckedBy    string    `json:"checked_by" db:"checked_by"`
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// CreateCheckinRequest for creating a new check-in
type CreateCheckinRequest struct {
	EventID      int64     `json:"event_id" binding:"required"`
	UserID       uuid.UUID `json:"user_id" binding:"required"`
	WalletAddress string   `json:"wallet_address" binding:"required"`
	QRCodeData   string    `json:"qr_code_data" binding:"required"`
	CheckedBy    string    `json:"checked_by" binding:"required"`
	IPAddress    string    `json:"ip_address"`
}

// EventParticipant represents participant information for events
type EventParticipant struct {
	UserID        uuid.UUID `json:"user_id"`
	WalletAddress string    `json:"wallet_address"`
	Name          string    `json:"name"`
	IsAttended    bool      `json:"is_attended"`
	StakeAmount   string    `json:"stake_amount"`
	RewardAmount  string    `json:"reward_amount"`
	Claimed       bool      `json:"claimed"`
	CheckedInAt   *time.Time `json:"checked_in_at"`
}

// GetEventParticipantsRequest for querying event participants
type GetEventParticipantsRequest struct {
	EventID     int64 `form:"event_id" binding:"required"`
	AttendedOnly bool `form:"attended_only"`
	Page        int   `form:"page,default=1"`
	Limit       int   `form:"limit,default=50"`
}

// SettleEventRequest for settling an event with attended participants
type SettleEventRequest struct {
	EventID              int64    `json:"event_id" binding:"required"`
	AttendedParticipants []string `json:"attended_participants" binding:"required"`
}

// NotifySettlementRequest for notifying about settlement
type NotifySettlementRequest struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// YieldDepositNotificationRequest for notifying about yield deposits
type YieldDepositNotificationRequest struct {
	TransactionHash string    `json:"transaction_hash"`
	Timestamp       time.Time `json:"timestamp"`
}

// StakesStats represents statistics for event stakes
type StakesStats struct {
	TotalParticipants      int     `json:"total_participants"`
	AttendedParticipants   int     `json:"attended_participants"`
	NoShowParticipants     int     `json:"no_show_participants"`
	TotalStaked            string  `json:"total_staked"`
	TotalRewards           string  `json:"total_rewards"`
	TotalClaimed           string  `json:"total_claimed"`
	AttendanceRate         float64 `json:"attendance_rate"`
}