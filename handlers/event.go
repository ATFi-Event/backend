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
	// Updated request structure to handle event metadata creation only
	// on-chain data should be handled by the indexer
	var req struct {
		EventID         int64  `json:"event_id" binding:"required"`
		Title           string `json:"title" binding:"required"`
		Description     string `json:"description"`
		ImageURL        string `json:"image_url"`
		OrganizerAddress string `json:"organizer_address"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Creating event metadata for EventID: %d, Title: %s, Organizer: %s", req.EventID, req.Title, req.OrganizerAddress)

	// Verify that on-chain data exists in events_onchain table (should be inserted by indexer)
	var onchainExists bool
	err := h.db.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM events_onchain WHERE event_id = $1)", req.EventID + 1).Scan(&onchainExists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify on-chain event data", "details": err.Error()})
		return
	}

	if !onchainExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "On-chain event data not found. Make sure the smart contract transaction is confirmed and indexed.", "event_id": req.EventID})
		return
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
		req.EventID + 1,
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
	log.Printf("Executing count query: %s with args: %v", countQuery, countArgs)
	err = h.db.QueryRow(c, countQuery, countArgs...).Scan(&total)
	if err != nil {
		log.Printf("Failed to get total count - Query: %s, Args: %v, Error: %v", countQuery, countArgs, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get total count", "details": err.Error()})
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

	// Get participant count from smart contractFailed to get total count
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

	// Check if event exists and get current status
	var status string
	query := `
		SELECT status
		FROM events_metadata
		WHERE event_id = $1
	`

	err := h.db.QueryRow(c, query, eventID).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if status != "LIVE" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event is not live"})
		return
	}

	// Update event status
	updateQuery := `
		UPDATE events_metadata
		SET status = 'SETTLED', updated_at = $1
		WHERE event_id = $2
	`

	_, err = h.db.Exec(c, updateQuery, time.Now(), eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update event status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event settled successfully"})
}

// ConfirmSettlement handles confirmation from frontend after successful blockchain settlement
func (h *EventHandler) ConfirmSettlement(c *gin.Context) {
	eventID := c.Param("id")

	var req struct {
		TransactionHash string        `json:"transaction_hash" binding:"required"`
		AttendedParticipants []string `json:"attended_participants" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Confirming settlement for event %s: tx=%s, participants=%d",
		eventID, req.TransactionHash, len(req.AttendedParticipants))

	// Update event status to SETTLED in events_metadata table
	updateQuery := `
		UPDATE events_metadata
		SET status = 'SETTLED', updated_at = $1
		WHERE event_id = $2
	`

	_, err := h.db.Exec(c, updateQuery, time.Now(), eventID)
	if err != nil {
		log.Printf("Database error updating event %s: %v", eventID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update event status", "details": err.Error()})
		return
	}

	log.Printf("Successfully updated event %s status to SETTLED", eventID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Event settlement confirmed successfully",
		"transaction_hash": req.TransactionHash,
	})
}

func (h *EventHandler) RegisterUser(c *gin.Context) {
	var req struct {
		EventID        int64  `json:"event_id" binding:"required"`
		UserAddress    string `json:"user_address" binding:"required"`
		TransactionHash string `json:"transaction_hash" binding:"required"`
		DepositAmount  string `json:"deposit_amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Registering user for event %d: address=%s, tx=%s, amount=%s",
		req.EventID, req.UserAddress, req.TransactionHash, req.DepositAmount)

	// Get user ID from profiles table using wallet address
	var userID *string
	err := h.db.QueryRow(c, "SELECT id FROM profiles WHERE wallet_address = $1", req.UserAddress).Scan(&userID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error querying user profile: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error when checking user profile"})
		return
	}

	// If user doesn't exist in profiles, create a basic profile
	if userID == nil {
		userID = new(string)
		insertProfileQuery := `
			INSERT INTO profiles (wallet_address, created_at, updated_at)
			VALUES ($1, $2, $3)
			RETURNING id
		`
		err = h.db.QueryRow(c, insertProfileQuery, req.UserAddress, time.Now(), time.Now()).Scan(userID)
		if err != nil {
			log.Printf("Error creating user profile: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user profile"})
			return
		}
		log.Printf("Created new profile for user %s with ID %s", req.UserAddress, *userID)
	}

	// Check if participant already exists
	var existingParticipant int
	err = h.db.QueryRow(c, "SELECT COUNT(*) FROM participant WHERE event_id = $1 AND user_id = $2", req.EventID, *userID).Scan(&existingParticipant)
	if err != nil {
		log.Printf("Error checking existing participant: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if existingParticipant > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Already registered for this event"})
		return
	}

	// Create participant record
	insertQuery := `
		INSERT INTO participant (event_id, user_id, is_attend, is_claim, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
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
	err = h.db.QueryRow(c, insertQuery, req.EventID, *userID, false, false, now, now).Scan(
		&participant.ID,
		&participant.EventID,
		&participant.UserID,
		&participant.IsAttend,
		&participant.IsClaim,
		&participant.CreatedAt,
		&participant.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error creating participant record: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register participant"})
		return
	}

	// Log the transaction for record keeping
	log.Printf("Participant registered: event=%d, user=%s, tx=%s", req.EventID, req.UserAddress, req.TransactionHash)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Successfully registered for event",
		"participant": participant,
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
		SELECT pr.wallet_address
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