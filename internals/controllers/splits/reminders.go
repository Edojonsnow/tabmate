package splitcontroller

import (
	"fmt"
	"log"
	"net/http"
	"tabmate/internals/notifications"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type RemindRequest struct {
	UserID string `json:"user_id"` // optional — if empty, remind all unsettled members
}

func RemindMembers(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		requesterID, _ := c.Get("user_id")
		pgRequesterID := requesterID.(pgtype.UUID)

		// Parse optional target user
		var req RemindRequest
		c.ShouldBindJSON(&req) // non-binding — body is optional

		// Fetch split
		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		// Only the host can send reminders
		hostMember, err := queries.GetSplitMember(c, tabmate.GetSplitMemberParams{
			SplitID: split.ID,
			UserID:  pgRequesterID,
		})
		if err != nil || hostMember.Role != "host" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only the split host can send reminders"})
			return
		}

		// Get host name for the notification body
		hostUser, err := queries.GetUserByID(c, pgRequesterID)
		hostName := "The host"
		if err == nil && hostUser.Name.Valid {
			hostName = hostUser.Name.String
		}

		// Fetch unsettled members with push tokens
		unsettled, err := queries.ListUnsettledSplitMembersForReminder(c, split.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch members"})
			return
		}

		if len(unsettled) == 0 {
			c.JSON(http.StatusOK, gin.H{"message": "No unsettled members to remind", "sent": 0})
			return
		}

		// If a specific user was requested, filter down to just them
		if req.UserID != "" {
			targetUUID, err := uuid.Parse(req.UserID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
				return
			}
			pgTarget := pgtype.UUID{Bytes: targetUUID, Valid: true}

			filtered := unsettled[:0]
			for _, m := range unsettled {
				if m.UserID == pgTarget {
					filtered = append(filtered, m)
					break
				}
			}
			if len(filtered) == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Member not found or already settled"})
				return
			}
			unsettled = filtered
		}

		totalAmountFloat, _ := split.TotalAmount.Float64Value()

		sent := 0
		for _, member := range unsettled {
			if !member.PushToken.Valid || member.PushToken.String == "" {
				continue
			}

			amountFloat, _ := member.AmountOwed.Float64Value()
			memberName := "there"
			if member.UserName.Valid {
				memberName = member.UserName.String
			}

			go func(token, name string, amount float64) {
				err := notifications.SendExpoPushNotification(notifications.ExpoMessage{
					To:    token,
					Title: "Payment reminder 💸",
					Body: fmt.Sprintf(
						"%s is reminding you to pay your share of \"%s\" ($%.2f of $%.2f total)",
						hostName, split.Name, amount, totalAmountFloat.Float64,
					),
					Data: map[string]string{
						"splitCode": split.SplitCode,
						"type":      "payment_reminder",
					},
				})
				if err != nil {
					log.Printf("Failed to send reminder to %s: %v", name, err)
				}
			}(member.PushToken.String, memberName, amountFloat.Float64)

			sent++
		}

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Reminders sent to %d member(s)", sent),
			"sent":    sent,
		})
	}
}
