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

// ReceiptItem is a single line item parsed from a receipt image.
type ReceiptItem struct {
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

// ParsedReceipt is the full structured output from a receipt scan.
type ParsedReceipt struct {
	Items []ReceiptItem `json:"items"`
	Tax   float64       `json:"tax"`
	Tip   float64       `json:"tip"`
}

type parsedReceiptWrapper struct {
	Items []ReceiptItem `json:"items"`
	Tax   float64       `json:"tax"`
	Tip   float64       `json:"tip"`
}

// ScanReceiptImage sends the image to Gemini and returns structured receipt data.
func ScanReceiptImage(imageBytes []byte, mediaType string) (*ParsedReceipt, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set")
	}

	encoded := base64.StdEncoding.EncodeToString(imageBytes)

	prompt := `You are a receipt parser. Extract all line items and totals from this receipt image.
Return ONLY valid JSON with no markdown formatting, no code blocks, no explanation.
Use exactly this structure:
{"items":[{"name":"Item Name","price":12.99,"quantity":1}],"tax":1.50,"tip":0}
Rules:
- Each item must have a name, a unit price (not the line total), and a quantity
- If quantity is not shown, default to 1
- If price is unclear, use 0
- "tax" is the tax/VAT/service charge total (not per-item). Use 0 if not present
- "tip" is the gratuity total. Use 0 if not present
- Do NOT include subtotal, total, tax, or tip as items — only food/drink line items
- Do NOT include any explanatory text outside the JSON`

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
					{Text: prompt},
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

	var parsed parsedReceiptWrapper
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse receipt JSON from gemini: %w, raw: %s", err, rawJSON)
	}

	return &ParsedReceipt{
		Items: parsed.Items,
		Tax:   parsed.Tax,
		Tip:   parsed.Tip,
	}, nil
}
