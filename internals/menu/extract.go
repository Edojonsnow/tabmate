package menu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const firecrawlAPI = "https://api.firecrawl.dev/v1/scrape"

// ExtractMenuFromURL scrapes a restaurant page via Firecrawl and parses items using Gemini.
func ExtractMenuFromURL(url string) ([]MenuItem, error) {
	markdown, err := scrapeWithFirecrawl(url)
	if err != nil {
		return nil, fmt.Errorf("scrape failed: %w", err)
	}
	return parseMenuFromText(markdown)
}

func scrapeWithFirecrawl(url string) (string, error) {
	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("FIRECRAWL_API_KEY not set")
	}

	body, _ := json.Marshal(map[string]any{
		"url":     url,
		"formats": []string{"markdown"},
	})

	req, err := http.NewRequest(http.MethodPost, firecrawlAPI, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("firecrawl request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("firecrawl error %d: %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Markdown string `json:"markdown"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("failed to parse firecrawl response: %w", err)
	}
	if !result.Success || result.Data.Markdown == "" {
		return "", fmt.Errorf("firecrawl returned empty content for this URL")
	}

	return result.Data.Markdown, nil
}

func parseMenuFromText(content string) ([]MenuItem, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set")
	}

	// Cap content to avoid hitting Gemini token limits
	if len(content) > 20000 {
		content = content[:20000]
	}

	prompt := fmt.Sprintf(`You are a menu parser. Extract all food and drink items from this restaurant menu text.
Return ONLY valid JSON with no markdown formatting, no code blocks, no explanation.
Use exactly this structure:
{"items":[{"name":"Item Name","price":12.99,"description":"Brief description or empty string","category":"Section name or empty string"}]}
Rules:
- If price is not visible or unclear, use 0
- If there is no description, use an empty string ""
- If the menu has named sections (e.g. Appetizers, Mains, Cocktails, Desserts), set category to that section name
- If there are no named sections, set category to ""
- All items within the same section must share the same category value
- Include every item you can read
- Do not include section headers as items, only actual food/drink items

Menu text:
%s`, content)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
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
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini error %d: %s", resp.StatusCode, string(respBytes))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	rawJSON := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)

	var parsed parsedMenu
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse menu JSON from gemini: %w", err)
	}

	return parsed.Items, nil
}
