package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
)

type DelayedAction struct {
	Id			string `form:"id" json:"id" binding:"optional"`
	ActionType	string `form:"actionType" json:"actionType" binding:"required"`
	ActionUrl	string `form:"actionUrl" json:"actionUrl" binding:"required"`
}

type EventDetails struct {
	Type 		string `form:"type" json:"type" binding:"required"`
	Uuid 		string `form:"uuid" json:"uuid" binding:"optional"`
	DeviceId	string `form:"uuid" json:"uuid" binding:"optional"`
	Tenant 		string `form:"tenantId" json:"tenantId" binding:"optional"`
	Action      DelayedAction `form:"action" json:"action" binding:"required"`
}

type ActionQuery struct {
	Uuid 		string 	`form:"uuid" json:"uuid" binding:"required"`
	DeviceId 	string	`form:"deviceId" json:"deviceId" binding:"required"`
	ApplicationVerison	string	`form:"appVersion" json:"appVersion" binding:"required"`
}

var delayedUserActions map[string][]DelayedAction
var delayedDeviceActions map[string][]DelayedAction

var correlationId string
var tenantId string

func extractIdentifyingHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationId = c.Request.Header.Get("X-MTT-Correlation-ID")
		if correlationId == "" {
			log.Printf("No correlation ID detected - Generating a lazy correlation ID\n")
			correlationId = "some-random-uuid-im-too-lazy-to-generate"
		}
		tenantId = c.Request.Header.Get("X-MTT-Tenant-ID")
	}
}

func cheapoLog(level string, msg string) {
	log.Printf("%s [correlationId: %s][tenantId: %s] %s - %s\n", time.Now().String(), correlationId, tenantId, level, msg)
}

func main() {
	port := os.Getenv("PORT")

	delayedUserActions = make(map[string][]DelayedAction)
	delayedDeviceActions = make(map[string][]DelayedAction)

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	router := gin.New()
	router.Use(gin.Logger())	//Log HTTP events
	router.Use(gin.Recovery())	//Handle errors
	router.Use(extractIdentifyingHeaders())	//Extract MTT headers for all routes

	// router.LoadHTMLGlob("templates/*.tmpl.html")
	// router.Static("/static", "static")

	// router.GET("/", func(c *gin.Context) {
	// 	c.HTML(http.StatusOK, "index.tmpl.html", nil)
	// })

	router.POST("/registerDelayedUserEvent", func(c *gin.Context) {
		var json EventDetails
		if err := c.BindJSON(&json); err != nil {
			cheapoLog("ERROR", "No JSON payload. Aborting request")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}

		if json.Uuid != "" {
			//User Action
			if delayedUserActions[json.Uuid] == nil {
				delayedUserActions[json.Uuid] = make([]DelayedAction)
			}

		}
	})

	router.POST("/queryDelayedActions", func(c *gin.Context) {

	})

	router.Run(":" + port)
}
