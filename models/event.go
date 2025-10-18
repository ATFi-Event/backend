package models

import (
	"time"
	"math/big"
	"github.com/google/uuid"
)

// Event status constants
const (
	StatusRegistrationOpen = "REGISTRATION_OPEN"
	StatusRegistrationClosed = "REGISTRATION_CLOSED"
	StatusLive = "LIVE"
	StatusSettled = "SETTLED"
	StatusVoided = "VOIDED"
)

// EventOnchain represents on-chain event data (matches new database schema)
type EventOnchain struct {
	EventID              int64      `json:"event_id" db:"event_id"`
	VaultAddress         string     `json:"vault_address" db:"vault_address"`
	OrganizerAddress     string     `json:"organizer_address" db:"organizer_address"`
	StakeAmount          string     `json:"stake_amount" db:"stake_amount"`
	MaxParticipants      int64      `json:"max_participant" db:"max_participant"`
	RegistrationDeadline int64      `json:"registration_deadline" db:"registration_deadline"` // Unix timestamp
	EventDate            int64      `json:"event_date" db:"event_date"`                    // Unix timestamp
}

// EventMetadata represents off-chain event metadata (matches actual database schema)
type EventMetadata struct {
	EventID    int64     `json:"event_id" db:"event_id"`
	Title      string    `json:"title" db:"title"`
	Status     string    `json:"status" db:"status"`
	Description *string   `json:"description,omitempty" db:"description"`
	ImageURL   *string   `json:"image_url,omitempty" db:"image_url"`
}

// EventDetail combines on-chain and off-chain data
type EventDetail struct {
	EventID            int64  `json:"event_id"`
	VaultAddress       string `json:"vault_address"`
	OrganizerAddress   string `json:"organizer_address"`
	StakeAmount        string `json:"stake_amount"`
	MaxParticipants    int64  `json:"max_participant"`
	CurrentParticipants int    `json:"current_participants"`
	RegistrationDeadline int64 `json:"registration_deadline"`
	EventDate          int64  `json:"event_date"`
	Title              string `json:"title"`
	Status             string `json:"status"`
	Description        *string `json:"description,omitempty"`
	ImageURL           *string `json:"image_url,omitempty"`
	OrganizerName      string `json:"organizer_name"`
}

// CreateEventMetadataRequest for creating off-chain metadata
type CreateEventMetadataRequest struct {
	EventID     int64  `json:"event_id" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
}

// UpdateEventMetadataRequest for updating metadata
type UpdateEventMetadataRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
}

// VaultYieldRecord tracks yield deposit operations
type VaultYieldRecord struct {
	ID                      uuid.UUID `json:"id" db:"id"`
	EventID                 int64     `json:"event_id" db:"event_id"`
	VaultAddress            string    `json:"vault_address" db:"vault_address"`
	DepositAmount           string    `json:"deposit_amount" db:"deposit_amount"`
	DepositTransactionHash  string    `json:"deposit_transaction_hash" db:"deposit_transaction_hash"`
	DepositTime             time.Time `json:"deposit_time" db:"deposit_time"`
	YieldProtocolUsed       string    `json:"yield_protocol_used" db:"yield_protocol_used"`
	CreatedAt               time.Time `json:"created_at" db:"created_at"`
}

// Legacy Event struct for backward compatibility
type Event struct {
	ID                   string    `json:"id" db:"event_id"`
	VaultAddress         string    `json:"vault_address" db:"vault_address"`
	OrganizerAddress     string    `json:"organizer_address" db:"organizer_address"`
	Name                 string    `json:"name" db:"title"`
	Description          string    `json:"description" db:"description"`
	Location             string    `json:"location" db:"location"`
	StakeAmount          *big.Int  `json:"stake_amount" db:"stake_amount"`
	RegistrationDeadline time.Time `json:"registration_deadline" db:"registration_deadline"`
	EventDate            time.Time `json:"event_date" db:"event_start_time"`
	MaxParticipants      int       `json:"max_participants" db:"max_participants"`
	Status               string    `json:"status" db:"status"`
	ImageURL             string    `json:"image_url" db:"image_url"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}

type CreateEventRequest struct {
	Name                 string    `json:"name" binding:"required"`
	Description          string    `json:"description"`
	Location             string    `json:"location"`
	StakeAmount          string    `json:"stake_amount" binding:"required"`
	RegistrationDeadline time.Time `json:"registration_deadline" binding:"required"`
	EventDate            time.Time `json:"event_date" binding:"required"`
	MaxParticipants      int       `json:"max_participants"`
	ImageURL             string    `json:"image_url"`
}

type EventStats struct {
	TotalParticipants   int     `json:"total_participants"`
	AttendedParticipants int    `json:"attended_participants"`
	TotalStakes         string  `json:"total_stakes"`
	TotalYield          string  `json:"total_yield"`
	IsSettled           bool    `json:"is_settled"`
}

type EventWithStats struct {
	*Event
	Stats *EventStats `json:"stats,omitempty"`
}