package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"
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
	var req struct {
		EventID int64  `json:"event_id" binding:"required"`
		UserID  string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Checking in participant: event=%d, user=%s", req.EventID, req.UserID)

	// Validate user ID is a valid UUID
	if _, err := uuid.Parse(req.UserID); err != nil {
		log.Printf("Invalid user ID format: %s: %v", req.UserID, err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid user ID format"})
		return
	}

	// Check if participant exists for this event
	var participantExists bool
	err := h.db.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM participant WHERE event_id = $1 AND user_id = $2)", req.EventID, req.UserID).Scan(&participantExists)
	if err != nil {
		log.Printf("Error checking participant existence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Database error"})
		return
	}

	if !participantExists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Participant not found for this event. Please ensure the participant has registered."})
		return
	}

	// Check if already checked in
	var isAttend bool
	err = h.db.QueryRow(c, "SELECT is_attend FROM participant WHERE event_id = $1 AND user_id = $2", req.EventID, req.UserID).Scan(&isAttend)
	if err != nil {
		log.Printf("Error checking attendance status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Database error"})
		return
	}

	if isAttend {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Participant has already checked in to this event"})
		return
	}

	// Update participant status to checked in
	updateQuery := `
		UPDATE participant
		SET is_attend = true, updated_at = $1
		WHERE event_id = $2 AND user_id = $3
		RETURNING id, event_id, user_id, is_attend, is_claim, created_at, updated_at
	`

	var participant struct {
		ID        string    `json:"id"`
		EventID   int64     `json:"event_id"`
		UserID    string    `json:"user_id"`
		IsAttend  bool      `json:"is_attend"`
		IsClaim   bool      `json:"is_claim"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	now := time.Now()
	err = h.db.QueryRow(c, updateQuery, now, req.EventID, req.UserID).Scan(
		&participant.ID,
		&participant.EventID,
		&participant.UserID,
		&participant.IsAttend,
		&participant.IsClaim,
		&participant.CreatedAt,
		&participant.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error updating participant check-in status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to check in participant"})
		return
	}

	log.Printf("Successfully checked in participant: event=%d, user=%s", req.EventID, req.UserID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully checked in to event",
		"participant": participant,
	})
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

	// If check-in is validated, also update participant table
	if req.IsValid {
		// Get user ID from profiles table using wallet address
		var userID uuid.UUID
		err = h.db.QueryRow(c, "SELECT id FROM profiles WHERE wallet_address = $1", checkin.UserAddress).Scan(&userID)
		if err != nil {
			// Log warning but don't fail the check-in validation
			log.Printf("Warning: Could not find user profile for wallet address %s: %v", checkin.UserAddress, err)
		} else {
			// Try to update existing participant record first
			updateQuery := `
				UPDATE participant
				SET is_attend = true
				WHERE user_id = $1
			`
			result, err := h.db.Exec(c, updateQuery, userID)
			if err != nil {
				log.Printf("Warning: Failed to update participant record for user %s: %v", userID, err)
			} else if result.RowsAffected() == 0 {
				// No existing record, try to insert (this will fail if either event_id or user_id already exists)
				insertQuery := `
					INSERT INTO participant (event_id, user_id, is_attend, is_claim)
					VALUES ($1, $2, true, false)
				`
				_, err = h.db.Exec(c, insertQuery, checkin.EventID, userID)
				if err != nil {
					log.Printf("Warning: Failed to insert participant record for event %s, user %s: %v", checkin.EventID, userID, err)
				} else {
					log.Printf("Successfully inserted participant record: event %s, user %s marked as attended", checkin.EventID, userID)
				}
			} else {
				log.Printf("Successfully updated participant record: user %s marked as attended", userID)
			}
		}
	}

	c.JSON(http.StatusOK, checkin)
}

// ClaimReward handles reward claiming for participants
func (h *CheckinHandler) ClaimReward(c *gin.Context) {
	var req struct {
		EventID int64  `json:"event_id" binding:"required"`
		UserID  string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Claiming reward for participant: event=%d, user=%s", req.EventID, req.UserID)

	// Get profile UUID using wallet address
	var profileUUID uuid.UUID
	var err error

	// Look up profile by wallet address to get the UUID
	err = h.db.QueryRow(c, "SELECT id FROM profiles WHERE wallet_address = $1", req.UserID).Scan(&profileUUID)
	if err != nil {
		log.Printf("Profile not found for wallet address %s: %v", req.UserID, err)
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "User profile not found. Please ensure you have a profile."})
		return
	}
	log.Printf("Found profile UUID %s for wallet address %s", profileUUID, req.UserID)

	// Check if participant exists for this event
	var participantExists bool
	err = h.db.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM participant WHERE event_id = $1 AND user_id = $2)", req.EventID, profileUUID).Scan(&participantExists)
	if err != nil {
		log.Printf("Error checking participant existence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Database error"})
		return
	}

	if !participantExists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Participant not found for this event. Please ensure you have registered."})
		return
	}

	// Get current participant status
	var isAttend, isClaim bool
	err = h.db.QueryRow(c, "SELECT is_attend, is_claim FROM participant WHERE event_id = $1 AND user_id = $2", req.EventID, profileUUID).Scan(&isAttend, &isClaim)
	if err != nil {
		log.Printf("Error checking participant status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Database error"})
		return
	}

	// Check if participant has checked in
	if !isAttend {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "You must check in to the event before claiming rewards"})
		return
	}

	// Check if reward already claimed
	if isClaim {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Reward has already been claimed for this event"})
		return
	}

	// Update participant status to claimed
	updateQuery := `
		UPDATE participant
		SET is_claim = true, updated_at = $1
		WHERE event_id = $2 AND user_id = $3
		RETURNING id, event_id, user_id, is_attend, is_claim, created_at, updated_at
	`

	var participant struct {
		ID        string    `json:"id"`
		EventID   int64     `json:"event_id"`
		UserID    string    `json:"user_id"`
		IsAttend  bool      `json:"is_attend"`
		IsClaim   bool      `json:"is_claim"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	now := time.Now()
	err = h.db.QueryRow(c, updateQuery, now, req.EventID, profileUUID).Scan(
		&participant.ID,
		&participant.EventID,
		&participant.UserID,
		&participant.IsAttend,
		&participant.IsClaim,
		&participant.CreatedAt,
		&participant.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error updating participant claim status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to claim reward"})
		return
	}

	log.Printf("Successfully claimed reward for participant: event=%d, user=%s", req.EventID, req.UserID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully claimed event reward",
		"participant": participant,
	})
}

// GetParticipantStatus retrieves participant status for an event
func (h *CheckinHandler) GetParticipantStatus(c *gin.Context) {
	eventIDParam := c.Param("id")
	userAddress := c.Param("userAddress")

	// Convert event ID to int64
	eventID, err := strconv.ParseInt(eventIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	log.Printf("Getting participant status: event=%d, user=%s", eventID, userAddress)

	// Get user ID from profiles table using wallet address
	var userID *string
	err = h.db.QueryRow(c, "SELECT id FROM profiles WHERE wallet_address = $1", userAddress).Scan(&userID)
	if err != nil {
		// Participant not found, return null
		c.JSON(http.StatusOK, gin.H{"participant": nil})
		return
	}

	// Get participant record
	var participant struct {
		ID        string    `json:"id"`
		EventID   int64     `json:"event_id"`
		UserID    string    `json:"user_id"`
		UserAddress string `json:"user_address"`
		IsAttend  bool      `json:"is_attend"`
		IsClaim   bool      `json:"is_claim"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	query := `
		SELECT p.id, p.event_id, p.user_id, pr.wallet_address, p.is_attend, p.is_claim, p.created_at, p.updated_at
		FROM participant p
		JOIN profiles pr ON p.user_id = pr.id
		WHERE p.event_id = $1 AND pr.wallet_address = $2
	`

	err = h.db.QueryRow(c, query, eventID, userAddress).Scan(
		&participant.ID,
		&participant.EventID,
		&participant.UserID,
		&participant.UserAddress,
		&participant.IsAttend,
		&participant.IsClaim,
		&participant.CreatedAt,
		&participant.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"participant": nil})
			return
		}
		log.Printf("Error getting participant status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"participant": participant})
}

// GetEventParticipants retrieves all participants for an event with profile information
func (h *CheckinHandler) GetEventParticipants(c *gin.Context) {
	eventIDParam := c.Param("id")

	// Convert event ID to int64
	eventID, err := strconv.ParseInt(eventIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	log.Printf("Getting participants for event: %d", eventID)

	// Get all participants with profile information
	query := `
		SELECT p.id, p.event_id, p.user_id, p.is_attend, p.is_claim, p.created_at, p.updated_at,
		       pr.wallet_address, pr.email, pr.name
		FROM participant p
		LEFT JOIN profiles pr ON p.user_id = pr.id
		WHERE p.event_id = $1
		ORDER BY p.created_at DESC
	`

	rows, err := h.db.Query(c, query, eventID)
	if err != nil {
		log.Printf("Error getting event participants: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var participants []struct {
		ID        string     `json:"id"`
		EventID   int64      `json:"event_id"`
		UserID    string     `json:"user_id"`
		IsAttend  bool       `json:"is_attend"`
		IsClaim   bool       `json:"is_claim"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		WalletAddress string    `json:"user_address"`
		Email     *string    `json:"email"`
		Name      *string    `json:"name"`
	}

	for rows.Next() {
		var participant struct {
			ID        string     `json:"id"`
			EventID   int64      `json:"event_id"`
			UserID    string     `json:"user_id"`
			IsAttend  bool       `json:"is_attend"`
			IsClaim   bool       `json:"is_claim"`
			CreatedAt time.Time  `json:"created_at"`
			UpdatedAt time.Time  `json:"updated_at"`
			WalletAddress string    `json:"user_address"`
			Email     *string    `json:"email"`
			Name      *string    `json:"name"`
		}

		err := rows.Scan(
			&participant.ID,
			&participant.EventID,
			&participant.UserID,
			&participant.IsAttend,
			&participant.IsClaim,
			&participant.CreatedAt,
			&participant.UpdatedAt,
			&participant.WalletAddress,
			&participant.Email,
			&participant.Name,
		)
		if err != nil {
			log.Printf("Error scanning participant row: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan participant data"})
			return
		}

		participants = append(participants, participant)
	}

	c.JSON(http.StatusOK, gin.H{
		"participants": participants,
		"count": len(participants),
	})
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