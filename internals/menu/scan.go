package menu

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const geminiAPI = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"

// MenuItem is a single parsed menu item returned to the client.
type MenuItem struct {
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
}

type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	GenerationConfig generationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type generationConfig struct {
	ResponseMimeType string `json:"response_mime_type"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type parsedMenu struct {
	Items []MenuItem `json:"items"`
}

// ScanMenuImage sends the image bytes to Gemini Flash and returns structured menu items.
func ScanMenuImage(imageBytes []byte, mediaType string) ([]MenuItem, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set")
	}

	encoded := base64.StdEncoding.EncodeToString(imageBytes)

	prompt := `You are a menu parser. Extract all food and drink items from this menu image.
		Return ONLY valid JSON with no markdown formatting, no code blocks, no explanation.
		Use exactly this structure:
		{"items":[{"name":"Item Name","price":12.99,"description":"Brief description or empty string"}]}
		Rules:
		- If price is not visible or unclear, use 0
		- If there is no description, use an empty string ""
		- Include every item you can read
		- Do not include section headers, only actual food/drink items`

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{
						InlineData: &inlineData{
							MimeType: mediaType,
							Data:     encoded,
						},
					},
					{
						Text: prompt,
					},
				},
			},
		},
		GenerationConfig: generationConfig{
			ResponseMimeType: "application/json",
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s?key=%s", geminiAPI, apiKey)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini api error %d: %s", resp.StatusCode, string(respBytes))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	rawJSON := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)

	var menu parsedMenu
	if err := json.Unmarshal([]byte(rawJSON), &menu); err != nil {
		return nil, fmt.Errorf("failed to parse menu JSON from gemini: %w, raw: %s", err, rawJSON)
	}

	return menu.Items, nil
}
