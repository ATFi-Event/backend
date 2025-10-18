package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"atfi-backend/models"
)

type EventHandler struct {
	db     *pgxpool.Pool
	client *ethclient.Client
}

func NewEventHandler(db *pgxpool.Pool, client *ethclient.Client) *EventHandler {
	return &EventHandler{
		db:     db,
		client: client,
	}
}

func (h *EventHandler) CreateEvent(c *gin.Context) {
	// Updated request structure to handle full event creation
	var req struct {
		EventID         int64  `json:"event_id" binding:"required"`
		Title           string `json:"title" binding:"required"`
		Description     string `json:"description"`
		Location        string `json:"location"`
		ImageURL        string `json:"image_url"`
		IsPublic        bool   `json:"is_public"`
		RequireApproval bool   `json:"require_approval"`
		OrganizerAddress string `json:"organizer_address"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Creating complete event for EventID: %d, Title: %s, Organizer: %s", req.EventID, req.Title, req.OrganizerAddress)

	// First, insert on-chain event data if not exists (from smart contract)
	// This would normally be handled by an indexer, but we'll insert it manually for now
	onchainQuery := `
		INSERT INTO events_onchain (event_id, vault_address, organizer_address, stake_amount, max_participant, registration_deadline, event_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (event_id) DO NOTHING
	`

	// For now, we'll use placeholder values since the indexer should handle this
	// In production, the vault address would come from the smart contract event
	_, err := h.db.Exec(c, onchainQuery,
		req.EventID,
		"0x0000000000000000000000000000000000000000", // Placeholder vault address
		req.OrganizerAddress,
		"0", // Placeholder stake amount
		0,   // Placeholder max participants
		0,   // Placeholder registration deadline
		0,   // Placeholder event date
	)

	if err != nil {
		log.Printf("Warning: Failed to insert on-chain data (may already exist): %v", err)
	}

	// Insert event metadata into database
	metadataQuery := `
		INSERT INTO events_metadata (event_id, title, description, image_url, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (event_id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			image_url = EXCLUDED.image_url,
			status = EXCLUDED.status
		RETURNING event_id, title, description, image_url, status
	`

	var metadata models.EventMetadata
	err = h.db.QueryRow(c, metadataQuery,
		req.EventID,
		req.Title,
		req.Description,
		req.ImageURL,
		"REGISTRATION_OPEN", // Initial status
	).Scan(
		&metadata.EventID,
		&metadata.Title,
		&metadata.Description,
		&metadata.ImageURL,
		&metadata.Status,
	)

	if err != nil {
		log.Printf("Failed to create event metadata: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create event metadata", "details": err.Error()})
		return
	}

	// Insert additional event details if we have a location or other fields
	if req.Location != "" {
		detailsQuery := `
			INSERT INTO event_details (event_id, location, is_public, require_approval)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (event_id) DO UPDATE SET
				location = EXCLUDED.location,
				is_public = EXCLUDED.is_public,
				require_approval = EXCLUDED.require_approval
		`

		_, err = h.db.Exec(c, detailsQuery,
			req.EventID,
			req.Location,
			req.IsPublic,
			req.RequireApproval,
		)

		if err != nil {
			log.Printf("Warning: Failed to insert event details: %v", err)
		}
	}

	// Return complete event detail including on-chain data
	var eventDetail models.EventDetail
	joinQuery := `
		SELECT
			eo.event_id, eo.vault_address, eo.organizer_address, eo.stake_amount,
			eo.max_participant, eo.registration_deadline, eo.event_date,
			em.title, em.description, em.image_url, em.status
		FROM events_onchain eo
		JOIN events_metadata em ON eo.event_id = em.event_id
		WHERE eo.event_id = $1
	`

	var description, imageURL *string
	var stakeAmountStr string

	err = h.db.QueryRow(c, joinQuery, req.EventID).Scan(
		&eventDetail.EventID,
		&eventDetail.VaultAddress,
		&eventDetail.OrganizerAddress,
		&stakeAmountStr,
		&eventDetail.MaxParticipants,
		&eventDetail.RegistrationDeadline,
		&eventDetail.EventDate,
		&eventDetail.Title,
		&description,
		&imageURL,
		&eventDetail.Status,
	)

	if err != nil {
		log.Printf("Failed to retrieve complete event details: %v", err)
		// Still return the metadata even if we can't get the full details
		c.JSON(http.StatusCreated, metadata)
		return
	}

	// Set additional fields
	eventDetail.StakeAmount = stakeAmountStr
	eventDetail.Description = description
	eventDetail.ImageURL = imageURL
	eventDetail.OrganizerName = "" // Default empty organizer name

	log.Printf("Successfully created complete event for EventID: %d", req.EventID)
	c.JSON(http.StatusCreated, eventDetail)
}

func (h *EventHandler) GetEvents(c *gin.Context) {
	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.Query("status")
	organizer := c.Query("organizer")

	offset := (page - 1) * limit

	// Build query using actual schema - join events_onchain and events_metadata
	query := `
		SELECT
			eo.event_id, eo.vault_address, eo.organizer_address, eo.stake_amount,
			eo.max_participant, eo.registration_deadline, eo.event_date,
			em.title, em.description, em.image_url, em.status
		FROM events_onchain eo
		JOIN events_metadata em ON eo.event_id = em.event_id
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	if status != "" {
		query += " AND em.status = $" + strconv.Itoa(argIndex)
		args = append(args, status)
		argIndex++
	}

	if organizer != "" {
		query += " AND eo.organizer_address = $" + strconv.Itoa(argIndex)
		args = append(args, organizer)
		argIndex++
	}

	query += " ORDER BY eo.event_id DESC LIMIT $" + strconv.Itoa(argIndex) + " OFFSET $" + strconv.Itoa(argIndex+1)
	args = append(args, limit, offset)

	log.Printf("Executing query: %s with args: %v", query, args)
	rows, err := h.db.Query(c, query, args...)
	if err != nil {
		log.Printf("Database query error in GetEvents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error", "details": err.Error()})
		return
	}
	defer rows.Close()

	var events []models.EventDetail
	for rows.Next() {
		var event models.EventDetail
		var stakeAmountStr string
		var description, imageURL *string

		err := rows.Scan(
			&event.EventID,
			&event.VaultAddress,
			&event.OrganizerAddress,
			&stakeAmountStr,
			&event.MaxParticipants,
			&event.RegistrationDeadline,
			&event.EventDate,
			&event.Title,
			&description,
			&imageURL,
			&event.Status,
		)
		if err != nil {
			log.Printf("Error scanning event row: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan event", "details": err.Error()})
			return
		}

		// Set additional fields
		event.StakeAmount = stakeAmountStr
		event.Description = description
		event.ImageURL = imageURL
		event.OrganizerName = "" // Default empty organizer name

		// Get current participants from smart contract if vault address exists
		var currentParticipants int64 = 0
		if event.VaultAddress != "" {
			if participantCount, err := h.getParticipantCountFromContract(event.VaultAddress); err == nil {
				currentParticipants = participantCount.Int64()
				log.Printf("Event %d has %d participants from contract", event.EventID, currentParticipants)
			} else {
				log.Printf("Failed to get participant count for event %d: %v", event.EventID, err)
			}
		}

		// Add current participants to the event response
		event.CurrentParticipants = int(currentParticipants)
		events = append(events, event)
	}

	// Get total count
	countQuery := `
		SELECT COUNT(*)
		FROM events_onchain eo
		JOIN events_metadata em ON eo.event_id = em.event_id
		WHERE 1=1
	`
	countArgs := []interface{}{}
	argIndex = 1

	if status != "" {
		countQuery += " AND em.status = $" + strconv.Itoa(argIndex)
		countArgs = append(countArgs, status)
		argIndex++
	}

	if organizer != "" {
		countQuery += " AND eo.organizer_address = $" + strconv.Itoa(argIndex)
		countArgs = append(countArgs, organizer)
		argIndex++
	}

	var total int
	err = h.db.QueryRow(c, countQuery, countArgs...).Scan(&total)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get total count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

func (h *EventHandler) GetEvent(c *gin.Context) {
	eventIDStr := c.Param("id")

	// Convert event ID to int64
	eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	// Query joining events_onchain and events_metadata
	query := `
		SELECT
			eo.event_id, eo.vault_address, eo.organizer_address, eo.stake_amount,
			eo.max_participant, eo.registration_deadline, eo.event_date,
			em.title, em.description, em.image_url, em.status
		FROM events_onchain eo
		JOIN events_metadata em ON eo.event_id = em.event_id
		WHERE eo.event_id = $1
	`

	var event models.EventDetail
	var stakeAmountStr string
	var description, imageURL *string

	err = h.db.QueryRow(c, query, eventID).Scan(
		&event.EventID,
		&event.VaultAddress,
		&event.OrganizerAddress,
		&stakeAmountStr,
		&event.MaxParticipants,
		&event.RegistrationDeadline,
		&event.EventDate,
		&event.Title,
		&description,
		&imageURL,
		&event.Status,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		log.Printf("Database query error in GetEvent: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Set additional fields
	event.StakeAmount = stakeAmountStr
	event.Description = description
	event.ImageURL = imageURL
	event.OrganizerName = "" // Default empty organizer name

	// Get participant count from smart contract
	if event.VaultAddress != "" {
		if participantCount, err := h.getParticipantCountFromContract(event.VaultAddress); err == nil {
			// Add participant count to response
			c.JSON(http.StatusOK, gin.H{
				"event":            event,
				"participant_count": participantCount.Int64(),
			})
			return
		} else {
			log.Printf("Failed to get participant count for event %d: %v", event.EventID, err)
		}
	}

	c.JSON(http.StatusOK, event)
}

func (h *EventHandler) SettleEvent(c *gin.Context) {
	eventID := c.Param("id")

	var req struct {
		AttendedParticipants []string `json:"attended_participants" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get event details
	var event models.Event
	query := `
		SELECT vault_address, organizer_address, status
		FROM events
		WHERE id = $1
	`

	err := h.db.QueryRow(c, query, eventID).Scan(
		&event.VaultAddress,
		&event.OrganizerAddress,
		&event.Status,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if event.Status != "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event is not active"})
		return
	}

	// TODO: Call smart contract to settle event
	// This would involve:
	// 1. Connecting to the vault contract
	// 2. Calling settleEvent with attended participants
	// 3. Updating the event status in database

	// Update event status
	updateQuery := `
		UPDATE events
		SET status = 'settled', updated_at = $1
		WHERE id = $2
	`

	_, err = h.db.Exec(c, updateQuery, time.Now(), eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update event status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event settled successfully"})
}

func (h *EventHandler) RegisterUser(c *gin.Context) {
	eventID := c.Param("id")

	var req struct {
		UserAddress string `json:"userAddress" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get event details
	var event models.Event
	query := `
		SELECT vault_address, registration_deadline, event_date, status
		FROM events
		WHERE id = $1
	`

	err := h.db.QueryRow(c, query, eventID).Scan(
		&event.VaultAddress,
		&event.RegistrationDeadline,
		&event.EventDate,
		&event.Status,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Check if registration is still open
	if time.Now().After(event.RegistrationDeadline) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Registration deadline has passed"})
		return
	}

	// Check if already registered
	var existingRegistration int
	err = h.db.QueryRow(c, "SELECT COUNT(*) FROM checkins WHERE event_id = $1 AND user_address = $2", eventID, req.UserAddress).Scan(&existingRegistration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if existingRegistration > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Already registered for this event"})
		return
	}

	// Create registration
	insertQuery := `
		INSERT INTO checkins (event_id, user_address, qr_data)
		VALUES ($1, $2, $3)
		RETURNING id, qr_data
	`

	var registration struct {
		ID    string `json:"id"`
		QRData string `json:"qr_data"`
	}

	qrData := fmt.Sprintf(`{"eventId":"%s","userAddress":"%s","timestamp":%d}`, eventID, req.UserAddress, time.Now().Unix())

	err = h.db.QueryRow(c, insertQuery, eventID, req.UserAddress, qrData).Scan(
		&registration.ID,
		&registration.QRData,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create registration"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"registration_id": registration.ID,
		"qr_data": registration.QRData,
		"message": "Successfully registered for event",
	})
}

func (h *EventHandler) GetUserRegistration(c *gin.Context) {
	eventID := c.Param("id")
	userAddress := c.Query("user")

	if userAddress == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User address is required"})
		return
	}

	var registration struct {
		ID              string    `json:"id"`
		EventID         string    `json:"event_id"`
		UserAddress     string    `json:"user_address"`
		QRData          string    `json:"qr_data"`
		CheckedInAt     time.Time `json:"checked_in_at"`
		IsValidated     bool      `json:"is_validated"`
		ValidatedAt     *time.Time `json:"validated_at"`
		ValidatedBy     *string   `json:"validated_by"`
	}

	query := `
		SELECT id, event_id, user_address, qr_data, checked_in_at, is_validated, validated_at, validated_by
		FROM checkins
		WHERE event_id = $1 AND user_address = $2
	`

	err := h.db.QueryRow(c, query, eventID, userAddress).Scan(
		&registration.ID,
		&registration.EventID,
		&registration.UserAddress,
		&registration.QRData,
		&registration.CheckedInAt,
		&registration.IsValidated,
		&registration.ValidatedAt,
		&registration.ValidatedBy,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Registration not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, registration)
}

func (h *EventHandler) NotifySettlement(c *gin.Context) {
	eventID := c.Param("id")

	var req struct {
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get event organizer
	var organizerAddress string
	err := h.db.QueryRow(c, "SELECT organizer_address FROM events WHERE id = $1", eventID).Scan(&organizerAddress)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// TODO: Send notification to organizer (email, push notification, etc.)
	// For now, just log the notification
	log.Printf("Settlement notification for event %s to organizer %s: %s", eventID, organizerAddress, req.Message)

	c.JSON(http.StatusOK, gin.H{"message": "Organizer notified about settlement"})
}

// PRD 2.5: Update event status
func (h *EventHandler) UpdateEventStatus(c *gin.Context) {
	eventID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate status
	validStatuses := []string{"REGISTRATION_OPEN", "REGISTRATION_CLOSED", "LIVE", "SETTLED", "VOIDED"}
	isValid := false
	for _, status := range validStatuses {
		if req.Status == status {
			isValid = true
			break
		}
	}

	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}

	// Update event status in events_metadata table
	updateQuery := `
		UPDATE events_metadata
		SET status = $1, updated_at = $2
		WHERE event_id = $3
	`

	result, err := h.db.Exec(c, updateQuery, req.Status, time.Now(), eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update event status"})
		return
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}

	log.Printf("Event %s status updated to %s", eventID, req.Status)

	c.JSON(http.StatusOK, gin.H{"message": "Event status updated successfully"})
}

// PRD 2.5: Get attended participants for event settlement
func (h *EventHandler) GetAttendedParticipants(c *gin.Context) {
	eventID := c.Param("id")

	query := `
		SELECT p.wallet_address
		FROM participant p
		JOIN profiles pr ON p.user_id = pr.id
		WHERE p.event_id = $1 AND p.is_attend = true
	`

	rows, err := h.db.Query(c, query, eventID)
	if err != nil {
		log.Printf("Database query error in GetAttendedParticipants: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var participants []string
	for rows.Next() {
		var walletAddress string
		err := rows.Scan(&walletAddress)
		if err != nil {
			log.Printf("Error scanning participant row: %v", err)
			continue
		}
		participants = append(participants, walletAddress)
	}

	c.JSON(http.StatusOK, participants)
}

// Helper function to get participant count from smart contract
func (h *EventHandler) getParticipantCountFromContract(vaultAddress string) (*big.Int, error) {
	if h.client == nil {
		return nil, fmt.Errorf("ethereum client not initialized")
	}

	// Simple ABI for getParticipantCount function
	vaultABI := `[{"inputs":[],"name":"getParticipantCount","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(vaultABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse vault ABI: %w", err)
	}

	// Pack the function call
	callData, err := parsedABI.Pack("getParticipantCount")
	if err != nil {
		return nil, fmt.Errorf("failed to pack call data: %w", err)
	}

	// Call the smart contract
	toAddress := common.HexToAddress(vaultAddress)
	result, err := h.client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &toAddress,
		Data: callData,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call getParticipantCount: %w", err)
	}

	// Unpack the result
	var participantCount *big.Int
	err = parsedABI.UnpackIntoInterface(&participantCount, "getParticipantCount", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	return participantCount, nil
}