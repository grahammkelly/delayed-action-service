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
	Id			string `form:"id" json:"id"`
	MinApplicationVersion string `form:"minAppVersion" json:"minAppVersion"`
	ActionType	string `form:"actionType" json:"actionType" binding:"required"`
	ActionUrl	string `form:"actionUrl" json:"actionUrl" binding:"required"`
}

type EventDetails struct {
	Type 		string `form:"type" json:"type" binding:"required"`
	Uuid 		string `form:"uuid" json:"uuid, omitempty"`
	DeviceId	string `form:"deviceId" json:"deviceId, omitempty"`
	Tenant 		string `form:"tenantId" json:"tenantId, omitempty"`
	Action      DelayedAction `form:"action" json:"action" binding:"required"`
}

type ActionQuery struct {
	Uuid 		string 	`form:"uuid" json:"uuid" binding:"required"`
	DeviceId 	string	`form:"deviceId" json:"deviceId" binding:"required"`
	ApplicationVersion	string	`form:"appVersion" json:"appVersion" binding:"required"`
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

	router.POST("/status", func(c *gin.Context) {
		cheapoLog("INFO", "Ya eejit. It's a GET.\tStatus - OK")
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
	})
	router.GET("/status", func(c *gin.Context) {
		cheapoLog("INFO", "Status - OK")
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
	})

	router.GET("/delayedActions", func(c *gin.Context) {
		switch strings.ToLower(c.Query("type")) {
		case "user" :
			c.JSON(http.StatusOK, gin.H{"userActions": delayedUserActions})
		case "device":
			c.JSON(http.StatusOK, gin.H{"deviceActions": delayedDeviceActions})
		default:
			c.JSON(http.StatusOK, gin.H{"userActions": delayedUserActions, "deviceActions": delayedDeviceActions})
		}
	})

	router.POST("/delayedAction", func(c *gin.Context) {
		var json EventDetails
		if err := c.BindJSON(&json); err != nil {
			cheapoLog("ERROR", "No JSON payload. Aborting request")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}

		newAction := json.Action

		//Cleanup the incoming action
		if newAction.Id == "" {
			newAction.Id = uuid.New().String()
		}
		if newAction.MinApplicationVersion == "" {
			newAction.MinApplicationVersion = "0"
		} else {
			newAction.MinApplicationVersion = strings.ToLower(newAction.MinApplicationVersion)
		}

		created := 0  //Currently don't support more than 1 at a time, but in the future .....?
		if json.Uuid != "" {
			cheapoLog("INFO",
				fmt.Sprintf("Adding USER action for user %s - (%v)", json.Uuid, newAction))

			//User Action
			userKey := fmt.Sprintf("%s##%s", tenantId, json.Uuid)
			delayedUserActions[userKey] = append(delayedUserActions[userKey], newAction)
			created ++
		} else if json.DeviceId != "" {
			cheapoLog("INFO",
				fmt.Sprintf("Adding DEVICE action for device %s - (%v)", json.DeviceId, newAction))

			//Device action
			deviceKey := fmt.Sprintf("%s##%s", tenantId, json.DeviceId)
			delayedDeviceActions[deviceKey] = append(delayedDeviceActions[deviceKey], newAction)
			created ++
		}

		c.JSON(http.StatusCreated, gin.H{"created": created})
	})

	router.POST("/delayedActions/find", func(c *gin.Context) {
		var qry ActionQuery
		if err := c.BindJSON(&qry); err != nil {
			cheapoLog("ERROR", "Query data invalid")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		qry.ApplicationVersion = strings.ToLower(qry.ApplicationVersion)	//Make sure any characters will be the same case when comparing

		cheapoLog("INFO", fmt.Sprintf("Searching for delayed actions for [%v]", qry))

		var matchedActions []DelayedAction	//Set up the array that will be returned

		//Check the user actions first
		if qry.Uuid != "" {
			var matchedUserActions []DelayedAction
			matchedUserActions, delayedUserActions = getMatchingActions(qry.Uuid, qry.ApplicationVersion, delayedUserActions)

			cheapoLog("DEBUG",
				fmt.Sprintf("Found %v user actions after filtering: %v", len(matchedUserActions), matchedUserActions))

			if len(matchedUserActions) > 0 {
				matchedActions = append(matchedActions, matchedUserActions...)
			}
		}

		if qry.DeviceId != "" {
			var matchedDeviceActions []DelayedAction
			matchedDeviceActions, delayedDeviceActions = getMatchingActions(qry.DeviceId, qry.ApplicationVersion, delayedDeviceActions)

			cheapoLog("DEBUG",
				fmt.Sprintf("Found %v device actions after filtering: %v", len(matchedDeviceActions), matchedDeviceActions))

			if len(matchedDeviceActions) > 0 {
				matchedActions = append(matchedActions, matchedDeviceActions...)
			}
		}

		//Return appropriate response
		if len(matchedActions) == 0 {	//No `(..?..:..)` in go
			c.JSON(http.StatusNotFound, []DelayedAction{})
		} else {
			c.JSON(http.StatusOK, matchedActions)
		}
	})

	router.DELETE("/delayedAction/:actionId", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
	})

	_ = router.Run(":" + port)
}

func getMatchingActions(id, appVersion string, delayedActions map[string][]DelayedAction) ([]DelayedAction, map[string][]DelayedAction) {
	var matchedActions []DelayedAction

	key := fmt.Sprintf("%s##%s", tenantId, id)
	if keyActions, ok := delayedActions[key]; ok {
		cheapoLog("DEBUG", fmt.Sprintf("Initial check matched %v actions...filtering those that apply", len(keyActions)))
		idx := 0
		for idx < len(keyActions) { //Can't loop over the slice because we're removing elements as we go!
			action := keyActions[idx]

			if action.MinApplicationVersion <= appVersion {
				cheapoLog("DEBUG", fmt.Sprintf("Action [%v] matched for App Version %s", action, appVersion))

				matchedActions = append(matchedActions, action)

				//Remove the action from the pending actions now
				keyActions[idx] = keyActions[len(keyActions)-1] //Swap this element with the last one
				keyActions = keyActions[:len(keyActions)-1]     //Remove the last element
			} else {
				idx++ //Only updating the idx we're looking at if we didn't update the underlying array
			}
		}

		cheapoLog("TRACE", fmt.Sprintf("Replacing actions for [%s]. \n\tWas: %v. \n\tReplacing with: %v", key, delayedActions[key], keyActions))
		delayedActions[key] = keyActions
	}

	return matchedActions, delayedActions

}
