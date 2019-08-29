package main

import (
	"log"
	"net/http"
	"os"
	"time"
	"strings"
	"fmt"

	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"

	"github.com/google/uuid"
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

func cheapoLog(level string, msg string) {
	log.Printf("%s [correlationId: %s][tenantId: %s] %s - %s\n", time.Now().String(), correlationId, tenantId, level, msg)
}

func extractIdentifyingHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantId = c.Request.Header.Get("X-MTT-Tenant-ID")
		if tenantId == "" {
			//Let's assume it's tripassist!
			tenantId = "tripassist"
			cheapoLog("WARN", "No Tenant id specified. Defaulting to 'tripassist'")
		}

		correlationId = c.Request.Header.Get("X-MTT-Correlation-ID")
		if correlationId == "" {
			correlationId = uuid.New().String()
			cheapoLog("TRACE", "No correlation ID detected - Generating a lazy correlation ID")
		}
	}
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

	router.GET("/status", func(c *gin.Context) {
		cheapoLog("INFO", "Status - OK")
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
	})

	router.POST("/registerDelayedAction", func(c *gin.Context) {
		var json EventDetails
		if err := c.BindJSON(&json); err != nil {
			cheapoLog("ERROR", "No JSON payload. Aborting request")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}

		newAction := json.Action
		if newAction.Id == "" {
			newAction.Id = uuid.New().String()
		}

		created := 0
		if json.Uuid != "" {
			cheapoLog("INFO",
				fmt.Sprintf("Adding USER action for user %s - %s (%s -> %s)", json.Uuid, newAction.Id, newAction.ActionType, newAction.ActionUrl))

			//User Action
			delayedUserActions[json.Uuid] = append(delayedUserActions[json.Uuid], json.Action)
			created += 1
		} else if json.DeviceId != "" {
			cheapoLog("INFO",
				fmt.Sprintf("Adding DEVICE action for user %s - %s (%s -> %s)", json.DeviceId, newAction.Id, newAction.ActionType, newAction.ActionUrl))
			delayedDeviceActions[json.DeviceId] = append(delayedDeviceActions[json.DeviceId], json.Action)
			created += 1
		}

		c.JSON(http.StatusCreated, gin.H{"created": created})
	})

	router.GET("/actions", func(c *gin.Context) {
		switch strings.ToLower(c.Query("type")) {
		case "user" :
			c.JSON(http.StatusOK, gin.H{"userActions": delayedUserActions})
		case "device":
			c.JSON(http.StatusOK, gin.H{"deviceActions": delayedDeviceActions})
		default:
			c.JSON(http.StatusOK, gin.H{"userActions": delayedUserActions, "deviceActions": delayedDeviceActions})
		}
	})

	//router.POST("/queryDelayedActions", func(c *gin.Context) {
	//
	//})

	_ = router.Run(":" + port)
}
