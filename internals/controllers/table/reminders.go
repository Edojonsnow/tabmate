package controllers

import (
	"fmt"
	"log"
	"net/http"
	"tabmate/internals/notifications"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func SendTablePaymentReminder(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		tableCode := c.Param("code")
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		dbTable, err := queries.GetTableByCode(c, tableCode)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Table not found"})
			return
		}

		if dbTable.CreatedBy != pgUserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only the table host can send reminders"})
			return
		}

		hostUser, err := queries.GetUserByID(c, pgUserID)
		hostName := "The host"
		if err == nil && hostUser.Name.Valid {
			hostName = hostUser.Name.String
		}

		tableName := tableCode
		if dbTable.Name.Valid {
			tableName = dbTable.Name.String
		}

		guests, err := queries.ListTableGuestsForReminder(c, tableCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch members"})
			return
		}

		if len(guests) == 0 {
			c.JSON(http.StatusOK, gin.H{"message": "No guests to remind", "sent": 0})
			return
		}

		sent := 0
		for _, g := range guests {
			if !g.PushToken.Valid || g.PushToken.String == "" {
				continue
			}
			token := g.PushToken.String
			name := "there"
			if g.UserName.Valid {
				name = g.UserName.String
			}
			go func(token, name string) {
				err := notifications.SendExpoPushNotification(notifications.ExpoMessage{
					To:    token,
					Title: "Time to pay up 💸",
					Body:  fmt.Sprintf("%s has finalised the bill for \"%s\". Check the app to see your share.", hostName, tableName),
					Data: map[string]string{
						"tableCode": tableCode,
						"type":      "payment_reminder",
					},
				})
				if err != nil {
					log.Printf("Failed to send table reminder to %s: %v", name, err)
				}
			}(token, name)
			sent++
		}

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Reminders sent to %d guest(s)", sent),
			"sent":    sent,
		})
	}
}
