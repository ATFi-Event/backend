package models

import (
	"time"
	"github.com/google/uuid"
)

type Profile struct {
	ID            uuid.UUID `json:"id" db:"id"`
	WalletAddress string    `json:"wallet_address" db:"wallet_address"`
	Name          *string   `json:"name" db:"name"`
	Email         *string   `json:"email" db:"email"`
	Balance       string    `json:"balance"` // Calculated from smart contract, not stored in DB
}

type CreateProfileRequest struct {
	WalletAddress string `json:"wallet_address" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Email         string `json:"email"`
}

type UpdateProfileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Legacy User struct for backward compatibility
type User struct {
	Address     string    `json:"address" db:"wallet_address"`
	Name        string    `json:"name" db:"name"`
	Email       string    `json:"email"`
	Avatar      string    `json:"avatar"`
	IsOrganizer bool      `json:"is_organizer"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type CreateUserRequest struct {
	Address     string `json:"address" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Email       string `json:"email"`
	Avatar      string `json:"avatar"`
	IsOrganizer bool   `json:"is_organizer"`
}

type UpdateUserRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Avatar      string `json:"avatar"`
	IsOrganizer bool   `json:"is_organizer"`
}