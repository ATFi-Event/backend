package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	. "atfi-backend/handlers"
)

func connectToDatabase() (*pgxpool.Pool, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@localhost/atfi_db?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}

	log.Println("Successfully connected to the database!")
	return pool, nil
}

// Added this function to connect to an Ethereum node, required by EventHandler
func connectToEthereum() (*ethclient.Client, error) {
    rpcURL := os.Getenv("RPC_URL")
    if rpcURL == "" {
        rpcURL = "https://base-sepolia-rpc.publicnode.com" // Default Base Sepolia RPC
    }

    client, err := ethclient.Dial(rpcURL)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
    }

    log.Println("Successfully connected to Ethereum node!")
    return client, nil
}


func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using default environment variables")
	}

	// Database connection
	pool, err := connectToDatabase()
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

    // Ethereum client connection
    ethClient, err := connectToEthereum()
    if err != nil {
        log.Fatalf("Unable to connect to Ethereum node: %v\n", err)
    }
    defer ethClient.Close()

	// Create handlers
	userHandler := NewUserHandler(pool, ethClient)
    eventHandler := NewEventHandler(pool, ethClient)
    checkinHandler := NewCheckinHandler(pool)


	// Setup Gin
	router := gin.Default()

	// CORS configuration
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:3000", "http://localhost:3001", "http://localhost:3002"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	router.Use(cors.New(corsConfig))

	// API routes
	api := router.Group("/api/v1")
	{
		// Profile routes
		api.POST("/profiles", userHandler.CreateProfile)
		api.GET("/profiles/:walletAddress", userHandler.GetProfile)
		api.PUT("/profiles/:walletAddress", userHandler.UpdateProfile)
		api.POST("/profiles/upsert", userHandler.UpsertProfile)

		// Event routes
        api.POST("/events", eventHandler.CreateEvent)
        api.GET("/events", eventHandler.GetEvents)
        api.GET("/events/:id", eventHandler.GetEvent)
        api.PUT("/events/:id/status", eventHandler.UpdateEventStatus)
        api.POST("/events/:id/settle", eventHandler.SettleEvent)
        api.POST("/events/:id/notify-settlement", eventHandler.NotifySettlement)
        api.GET("/events/:id/attended", eventHandler.GetAttendedParticipants)
        
        // Event registration routes
        api.POST("/events/:id/register", eventHandler.RegisterUser)
        api.GET("/events/:id/registration", eventHandler.GetUserRegistration)

		// Checkin routes
        api.POST("/checkin", checkinHandler.CheckIn)
        api.POST("/checkin/validate", checkinHandler.ValidateCheckIn)
        api.GET("/events/:id/checkins", checkinHandler.GetCheckins)

		// Health check route
		api.GET("/test-db", func(c *gin.Context) {
			err := pool.Ping(context.Background())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "Database connection OK"})
		})
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s\n", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v\n", err)
	}
}