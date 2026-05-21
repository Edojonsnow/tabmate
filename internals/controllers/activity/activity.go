package activity

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	tabmate "tabmate/internals/store/postgres"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ActivityEventResponse is the JSON shape returned to the mobile client.
type ActivityEventResponse struct {
	ID         string          `json:"id"`
	EventType  string          `json:"event_type"`
	ActorName  string          `json:"actor_name"`
	EntityType string          `json:"entity_type"`
	EntityCode string          `json:"entity_code"`
	EntityName string          `json:"entity_name"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
}

func toResponse(e tabmate.ActivityEvents) ActivityEventResponse {
	metadata := json.RawMessage(e.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	var createdAt time.Time
	if e.CreatedAt.Valid {
		createdAt = e.CreatedAt.Time
	}
	return ActivityEventResponse{
		ID:         uuid.UUID(e.ID.Bytes).String(),
		EventType:  e.EventType,
		ActorName:  e.ActorName,
		EntityType: e.EntityType,
		EntityCode: e.EntityCode,
		EntityName: e.EntityName,
		Metadata:   metadata,
		CreatedAt:  createdAt,
	}
}

// InsertEvent writes an activity event in a fire-and-forget manner.
// Errors are logged but never propagate to the caller.
func InsertEvent(ctx context.Context, queries tabmate.Querier, params tabmate.InsertActivityEventParams) {
	if len(params.Metadata) == 0 {
		params.Metadata = []byte("{}")
	}
	if _, err := queries.InsertActivityEvent(ctx, params); err != nil {
		log.Printf("[activity] failed to insert event %q for entity %s: %v", params.EventType, params.EntityCode, err)
	}
}

// GetActivityFeed returns the 50 most recent events for the authenticated user.
func GetActivityFeed(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
			return
		}
		pgUserID := userID.(pgtype.UUID)

		events, err := queries.ListActivityEventsForUser(c, pgUserID)
		if err != nil {
			log.Printf("[activity] ListActivityEventsForUser error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activity feed"})
			return
		}

		resp := make([]ActivityEventResponse, len(events))
		for i, e := range events {
			resp[i] = toResponse(e)
		}
		c.JSON(http.StatusOK, resp)
	}
}
