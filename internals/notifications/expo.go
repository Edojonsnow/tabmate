package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type ExpoMessage struct {
	To    string            `json:"to"`
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data,omitempty"`
}

func SendExpoPushNotification(msg ExpoMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	resp, err := http.Post(
		"https://exp.host/--/api/v2/push/send",
		"application/json",
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("expo push API returned status %d", resp.StatusCode)
	}

	return nil
}
