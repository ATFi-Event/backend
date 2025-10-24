package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"atfi-backend/models"
)

type UserHandler struct {
	db     *pgxpool.Pool
	client *ethclient.Client
}

func NewUserHandler(db *pgxpool.Pool, client *ethclient.Client) *UserHandler {
	return &UserHandler{
		db:     db,
		client: client,
	}
}

// Profile handlers using profiles table
func (h *UserHandler) CreateProfile(c *gin.Context) {
	var req models.CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if profile already exists
	var exists bool
	err := h.db.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM profiles WHERE wallet_address = $1)", req.WalletAddress).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check if profile exists" + err.Error(),})
		return
	}

	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Profile already exists"})
		return
	}

	// Create profile
	query := `
		INSERT INTO profiles (id, wallet_address, name, email)
		VALUES ($1, $2, $3, $4)
		RETURNING id, wallet_address, name, email
	`
	log.Printf("GetProfile called for wallet address: %s", req.Email)

	var profile models.Profile
	err = h.db.QueryRow(c, query,
		uuid.New(),
		req.WalletAddress,
		req.Name,
		req.Email,
	).Scan(
		&profile.ID,
		&profile.WalletAddress,
		&profile.Name,
		&profile.Email,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create profile: " + err.Error(),})
		return
	}

	profile.Balance = ""

	c.JSON(http.StatusCreated, profile)
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	walletAddress := c.Param("walletAddress")
	log.Printf("GetProfile called for wallet address: %s", walletAddress)

	var profile models.Profile
	query := `
		SELECT id, wallet_address, name, email
		FROM profiles
		WHERE wallet_address = $1
	`

	err := h.db.QueryRow(c, query, walletAddress).Scan(
		&profile.ID,
		&profile.WalletAddress,
		&profile.Name,
		&profile.Email,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Profile not found for wallet: %s", walletAddress)
			c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
			return
		}
		log.Printf("Database error getting profile for %s: %v", walletAddress, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error", "details": err.Error()})
		return
	}

	// Get USDC balance from smart contract
	balance := "0"
	if usdcBalance, err := h.getUSDCBalanceFromContract(walletAddress); err == nil {
		balance = usdcBalance
		log.Printf("Retrieved USDC balance for %s: %s", walletAddress, balance)
	} else {
		log.Printf("Failed to get USDC balance for %s: %v", walletAddress, err)
	}
	profile.Balance = balance

	// Convert nullable fields to strings for JSON response
	response := map[string]interface{}{
		"id":            profile.ID,
		"wallet_address": profile.WalletAddress,
		"name":          profile.Name,
		"email":         profile.Email,
		"balance":       profile.Balance,
	}

	c.JSON(http.StatusOK, response)
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	walletAddress := c.Param("walletAddress")

	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if profile exists
	var exists bool
	err := h.db.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM profiles WHERE wallet_address = $1)", walletAddress).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
		return
	}

	// Update profile - allow updating both name and email
	query := `
		UPDATE profiles
		SET name = COALESCE($2, name),
		    email = COALESCE($3, email)
		WHERE wallet_address = $1
		RETURNING id, wallet_address, name, email
	`

	var profile models.Profile
	err = h.db.QueryRow(c, query,
		walletAddress,
		nullIfEmpty(req.Name),
		nullIfEmpty(req.Email),
	).Scan(
		&profile.ID,
		&profile.WalletAddress,
		&profile.Name,
		&profile.Email,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	// Balance will be populated from smart contract in frontend
	profile.Balance = ""

	c.JSON(http.StatusOK, profile)
}

func (h *UserHandler) UpsertProfile(c *gin.Context) {
	var req models.CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if profile already exists
	var exists bool
	err := h.db.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM profiles WHERE wallet_address = $1)", req.WalletAddress).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if exists {
		// Update existing profile - allow updating both name and email
		query := `
			UPDATE profiles
			SET name = COALESCE($2, name),
			    email = COALESCE($3, email)
			WHERE wallet_address = $1
			RETURNING id, wallet_address, name, email
		`

		var profile models.Profile
		err = h.db.QueryRow(c, query,
			req.WalletAddress,
			nullIfEmpty(req.Name),
			nullIfEmpty(req.Email),
		).Scan(
			&profile.ID,
			&profile.WalletAddress,
			&profile.Name,
			&profile.Email,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}

		// Balance will be populated from smart contract in frontend
		profile.Balance = ""

		c.JSON(http.StatusOK, profile)
		return
	}

	// Create new profile
	h.CreateProfile(c)
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// Helper function to get USDC balance from smart contract
func (h *UserHandler) getUSDCBalanceFromContract(walletAddress string) (string, error) {
	if h.client == nil {
		return "0", fmt.Errorf("ethereum client not initialized")
	}

	// USDC contract address on Base Sepolia
	usdcAddress := "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
	log.Printf("Getting USDC balance for wallet %s from contract %s", walletAddress, usdcAddress)

	// ERC20 balanceOf function ABI
	erc20ABI := `[{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return "0", fmt.Errorf("failed to parse USDC ABI: %w", err)
	}

	// Validate and pack the function call with wallet address
	if !common.IsHexAddress(walletAddress) {
		return "0", fmt.Errorf("invalid wallet address: %s", walletAddress)
	}

	callData, err := parsedABI.Pack("balanceOf", common.HexToAddress(walletAddress))
	if err != nil {
		return "0", fmt.Errorf("failed to pack balanceOf call data: %w", err)
	}

	log.Printf("Calling USDC contract with data length: %d", len(callData))

	// Call the USDC smart contract
	toAddress := common.HexToAddress(usdcAddress)
	result, err := h.client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &toAddress,
		Data: callData,
	}, nil)
	if err != nil {
		return "0", fmt.Errorf("failed to call balanceOf: %w", err)
	}

	log.Printf("Contract call returned result length: %d", len(result))

	if len(result) == 0 {
		return "0", fmt.Errorf("empty result from contract call")
	}

	// Unpack the result
	var balance *big.Int
	err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return "0", fmt.Errorf("failed to unpack balanceOf result: %w", err)
	}

	log.Printf("Raw balance: %s", balance.String())

	// Convert from wei (6 decimals for USDC) to regular USDC
	balanceUSDC := new(big.Float).SetInt(balance)
	balanceUSDC.Quo(balanceUSDC, big.NewFloat(1000000)) // USDC has 6 decimals

	log.Printf("USDC balance for %s: %s", walletAddress, balanceUSDC.String())
	return balanceUSDC.String(), nil
}