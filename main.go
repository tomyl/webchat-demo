package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"gopkg.in/olahol/melody.v1"
)

type Message struct {
	gorm.Model
	Name    string `json:"name"`
	Message string `json:"message"`
}

func main() {
	// Open database
	db, err := gorm.Open("sqlite3", "messages.db")

	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	// Close database if exiting main
	defer db.Close()

	// Create the message table if missing
	db.AutoMigrate(&Message{})

	// Set up websocket handler
	hub := melody.New()

	hub.HandleMessage(func(s *melody.Session, buf []byte) {
		var msg Message

		if err := json.Unmarshal(buf, &msg); err != nil {
			log.Printf("Failed to parse message: %v\n%s\n\n", err, msg)
			return
		}

		log.Printf("Received message from %s: %s\n", msg.Name, msg.Message)

		// Store message in database
		if err := db.Create(&msg).Error; err != nil {
			log.Printf("Failed to save message: %v\n", err)
			// XXX: Fall through!
		}

		// Message now has a timestamp after being saved to database.
		// Reserialize message and broadcast to all clients.
		newbuf, err := json.Marshal(msg)

		if err != nil {
			log.Printf("Failed to encode message: %v\n%#v\n\n", err, msg)
			return
		}

		hub.Broadcast(newbuf)
	})

	// Set up http resource router
	router := gin.Default()

	// Serve static files
	router.Static("/static", "./static")

	// Serve index.html at root for convenience
	router.GET("/", func(c *gin.Context) {
		http.ServeFile(c.Writer, c.Request, "./static/index.html")
	})

	// Accept websocket connections
	router.GET("/ws", func(c *gin.Context) {
		hub.HandleRequest(c.Writer, c.Request)
	})

	// Allow clients to fetch history
	router.GET("/history", func(c *gin.Context) {
		var messages []Message

		if err := db.Limit(10).Find(&messages).Error; err != nil {
			c.String(http.StatusInternalServerError, "Failed to fetch history")
			return
		}

		c.JSON(http.StatusOK, messages)
	})

	// Serve forever
	router.Run(":5000")
}
