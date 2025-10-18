package models

import (
	"time"
)

type CheckIn struct {
	ID        string    `json:"id" db:"id"`
	EventID   string    `json:"event_id" db:"event_id"`
	UserAddress string  `json:"user_address" db:"user_address"`
	QRData    string    `json:"qr_data" db:"qr_data"`
	CheckedInAt time.Time `json:"checked_in_at" db:"checked_in_at"`
	IsValidated bool   `json:"is_validated" db:"is_validated"`
	ValidatedAt  *time.Time `json:"validated_at,omitempty" db:"validated_at"`
	ValidatedBy  string  `json:"validated_by,omitempty" db:"validated_by"`
}

type CheckInRequest struct {
	EventID    string `json:"event_id" binding:"required"`
	QRData     string `json:"qr_data" binding:"required"`
	UserAddress string `json:"user_address" binding:"required"`
}

type ValidateCheckInRequest struct {
	CheckInID string `json:"checkin_id" binding:"required"`
	IsValid   bool   `json:"is_valid"`
}