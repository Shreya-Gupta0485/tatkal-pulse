package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	rdb    *redis.Client
	luaSHA string
	ctx    = context.Background()
)

type BookingRequest struct {
	TrainNumber string `json:"train_no" binding:"required"`
	Passenger   string `json:"passenger" binding:"required"`
}

func main() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		PoolSize: 100,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ Redis connection failed: %v", err)
	}
	fmt.Println("✅ Redis connected successfully!")

	scriptBytes, err := os.ReadFile("scripts/reserve.lua")
	if err != nil {
		log.Fatalf("❌ scripts/reserve.lua not found: %v", err)
	}

	luaSHA, err = rdb.ScriptLoad(ctx, string(scriptBytes)).Result()
	if err != nil {
		log.Fatalf("❌ Lua script load failed: %v", err)
	}
	fmt.Println("✅ Atomic Tatkal Lua script pre-loaded in Redis!")

	resetTatkalQuota()

	router := gin.Default()

	// CORS Setup
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// 👉 Bulletproof File Handler (No 404!)
	router.GET("/", func(c *gin.Context) {
		content, err := os.ReadFile("frontend/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "❌ File path error: %v", err)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	router.POST("/api/v1/book", handleTatkalBooking)
	router.POST("/api/v1/reset", handleReset)

	fmt.Println("🚄 IRCTC Tatkal Engine running on http://localhost:8081")
	router.Run(":8081")
}

func resetTatkalQuota() {
	rdb.Set(ctx, "train:22436:tatkal_seats", 1, 0)
	rdb.Del(ctx, "train:22436:passengers")
	fmt.Println("🎫 Tatkal Quota reset: Train 22436 has exactly 1 SEAT remaining!")
}

func handleReset(c *gin.Context) {
	resetTatkalQuota()
	c.JSON(http.StatusOK, gin.H{
		"status":  "RESET",
		"message": "Tatkal Quota successfully reset to 1 seat!",
	})
}

func handleTatkalBooking(c *gin.Context) {
	var req BookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid booking payload"})
		return
	}

	pnr := "PNR-" + uuid.New().String()[:8]
	seatKey := fmt.Sprintf("train:%s:tatkal_seats", req.TrainNumber)
	passengersKey := fmt.Sprintf("train:%s:passengers", req.TrainNumber)

	res, err := rdb.EvalSha(ctx, luaSHA, []string{seatKey, passengersKey}, req.Passenger, pnr).Int()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	switch res {
	case 1:
		c.JSON(http.StatusOK, gin.H{
			"status":    "CONFIRMED",
			"pnr":       pnr,
			"berth":     "B1 - Berth 42 (Lower)",
			"passenger": req.Passenger,
			"message":   "🎉 TICKET CONFIRMED (CNF)! Seat allocated.",
		})
	case 0:
		c.JSON(http.StatusConflict, gin.H{
			"status":    "REGRET",
			"passenger": req.Passenger,
			"message":   "😔 REGRET / NOT AVAILABLE: All Tatkal seats exhausted!",
		})
	case -1:
		c.JSON(http.StatusForbidden, gin.H{
			"status":    "ALREADY_BOOKED",
			"passenger": req.Passenger,
			"message":   "⚠️ Duplicate booking detected for passenger.",
		})
	}
}