package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
	"atfi-backend/models"
)

type CheckinHandler struct {
	db *pgxpool.Pool
}

func NewCheckinHandler(db *pgxpool.Pool) *CheckinHandler {
	return &CheckinHandler{db: db}
}

func (h *CheckinHandler) CheckIn(c *gin.Context) {
	var req models.CheckInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify event exists and is active
	var eventStatus string
	err := h.db.QueryRow(c, "SELECT status FROM events WHERE id = $1", req.EventID).Scan(&eventStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if eventStatus != "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event is not active"})
		return
	}

	// Generate unique QR data if not provided
	qrData := req.QRData
	if qrData == "" {
		qrData = generateQRData(req.UserAddress, req.EventID)
	}

	// Check if user already checked in
	var exists bool
	err = h.db.QueryRow(c,
		"SELECT EXISTS(SELECT 1 FROM checkins WHERE event_id = $1 AND user_address = $2)",
		req.EventID, req.UserAddress).Scan(&exists)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "User already checked in"})
		return
	}

	// Create check-in record
	checkInID := uuid.New().String()
	query := `
		INSERT INTO checkins (id, event_id, user_address, qr_data, checked_in_at, is_validated)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, event_id, user_address, qr_data, checked_in_at, is_validated
	`

	var checkin models.CheckIn
	err = h.db.QueryRow(c, query,
		checkInID,
		req.EventID,
		req.UserAddress,
		qrData,
		time.Now(),
		false, // Not validated yet
	).Scan(
		&checkin.ID,
		&checkin.EventID,
		&checkin.UserAddress,
		&checkin.QRData,
		&checkin.CheckedInAt,
		&checkin.IsValidated,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create check-in"})
		return
	}

	c.JSON(http.StatusCreated, checkin)
}

func (h *CheckinHandler) GetCheckins(c *gin.Context) {
	eventID := c.Param("id")

	// Verify event exists
	var exists bool
	err := h.db.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM events WHERE id = $1)", eventID).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}

	// Get check-ins
	query := `
		SELECT id, event_id, user_address, qr_data, checked_in_at, is_validated, validated_at, validated_by
		FROM checkins
		WHERE event_id = $1
		ORDER BY checked_in_at DESC
	`

	rows, err := h.db.Query(c, query, eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var checkins []models.CheckIn
	for rows.Next() {
		var checkin models.CheckIn
		err := rows.Scan(
			&checkin.ID,
			&checkin.EventID,
			&checkin.UserAddress,
			&checkin.QRData,
			&checkin.CheckedInAt,
			&checkin.IsValidated,
			&checkin.ValidatedAt,
			&checkin.ValidatedBy,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan check-in"})
			return
		}

		checkins = append(checkins, checkin)
	}

	c.JSON(http.StatusOK, checkins)
}

func (h *CheckinHandler) ValidateCheckIn(c *gin.Context) {
	var req models.ValidateCheckInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get organizer address from context (assuming authenticated)
	organizerAddress := c.GetString("user_address")
	if organizerAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Verify check-in exists and get event details
	var eventID string
	err := h.db.QueryRow(c, "SELECT event_id FROM checkins WHERE id = $1", req.CheckInID).Scan(&eventID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Check-in not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Verify organizer owns the event
	var organizer string
	err = h.db.QueryRow(c, "SELECT organizer_address FROM events WHERE id = $1", eventID).Scan(&organizer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if organizer != organizerAddress {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to validate this check-in"})
		return
	}

	// Update check-in validation
	query := `
		UPDATE checkins
		SET is_validated = $1, validated_at = $2, validated_by = $3
		WHERE id = $4
		RETURNING id, event_id, user_address, qr_data, checked_in_at, is_validated, validated_at, validated_by
	`

	var checkin models.CheckIn
	var validatedAt time.Time
	if req.IsValid {
		now := time.Now()
		validatedAt = now
	}

	err = h.db.QueryRow(c, query,
		req.IsValid,
		validatedAt,
		organizerAddress,
		req.CheckInID,
	).Scan(
		&checkin.ID,
		&checkin.EventID,
		&checkin.UserAddress,
		&checkin.QRData,
		&checkin.CheckedInAt,
		&checkin.IsValidated,
		&checkin.ValidatedAt,
		&checkin.ValidatedBy,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update check-in"})
		return
	}

	c.JSON(http.StatusOK, checkin)
}

// generateQRData generates unique QR data for check-in
func generateQRData(userAddress, eventID string) string {
	// Generate random bytes
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)

	// Create QR data: userAddress:eventID:randomSuffix
	qrData := userAddress + ":" + eventID + ":" + hex.EncodeToString(randomBytes)

	return qrData
}