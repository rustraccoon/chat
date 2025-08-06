package llama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// LlamaService provides methods for interacting with the Llama API.
type LlamaService struct{}

// GroqMessage defines the structure for a message in the chat history.
type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GroqRequest defines the structure for the API request payload.
type GroqRequest struct {
	Model       string        `json:"model"`
	Messages    []GroqMessage `json:"messages"`
	Temperature float32       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	TopP        float32       `json:"top_p,omitempty"`
	Stream      bool          `json:"stream"` // Set to false for non-streaming responses
}

// GroqResponse defines the structure for a non-streaming API response.
type GroqResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice defines the structure for a single choice in the response.
type Choice struct {
	Index        int          `json:"index"`
	Message      *GroqMessage `json:"message"` // Changed to a pointer to handle potential nil messages
	FinishReason string       `json:"finish_reason"`
}

// Usage defines the token usage statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Prompt holds the system and user messages.
type Prompt struct {
	System string
	User   string
}

// Ask sends a prompt to the Groq API and returns the complete response.
func (l LlamaService) Ask(prompt Prompt) (string, error) {
	// Retrieve API configuration from environment variables.
	groqApiURL := os.Getenv("GROQ_API_URL")
	groqApiKey := os.Getenv("GROQ_API_KEY")
	groqModel := os.Getenv("GROQ_MODEL")

	// Validate environment variables.
	if groqApiKey == "" || groqApiURL == "" || groqModel == "" {
		return "", fmt.Errorf("one or more GROQ environment variables are not set: GROQ_API_URL, GROQ_API_KEY, GROQ_MODEL")
	}

	// Construct the request payload.
	reqBody := GroqRequest{
		Model: groqModel,
		Messages: []GroqMessage{
			{Role: "system", Content: prompt.System},
			{Role: "user", Content: prompt.User},
		},
		Temperature: 0.7,
		MaxTokens:   1024,
		TopP:        1.0,
		Stream:      false, // IMPORTANT: Set to false for non-streaming
	}

	// Marshal the request body to JSON.
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create a new HTTP POST request.
	req, err := http.NewRequest("POST", groqApiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create new http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+groqApiKey)

	// Send the request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request to Groq API: %w", err)
	}
	defer resp.Body.Close() // Ensure the response body is closed.

	// Check for non-OK HTTP status codes.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("groq API error: status code %d, body: %s", resp.StatusCode, string(body))
	}

	// Decode the non-streaming JSON response.
	var groqResponse GroqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResponse); err != nil {
		return "", fmt.Errorf("failed to decode groq API response: %w", err)
	}

	// Add robust checking to prevent panic if no choices are returned.
	if len(groqResponse.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from Groq API")
	}

	choice := groqResponse.Choices[0]

	// A successful response will typically have a finish_reason of "stop".
	// Other reasons like "length" (max tokens reached) or "content_filter"
	// might result in a nil or incomplete message object.
	// By checking for "stop" and then for a non-nil message, we ensure safe access.
	if choice.FinishReason == "stop" {
		if choice.Message == nil {
			return "", fmt.Errorf("groq API returned a choice with 'stop' reason but a nil message object")
		}
		return choice.Message.Content, nil
	}

	// If the finish reason is not "stop", the message is incomplete or filtered.
	// We also check if the message is nil in this case for extra safety.
	if choice.Message == nil {
		return "", fmt.Errorf("groq API returned a non-terminal choice with nil message. finish_reason: '%s'", choice.FinishReason)
	}

	return "", fmt.Errorf("groq API returned a non-terminal choice. finish_reason: '%s', partial_content: '%s'", choice.FinishReason, choice.Message.Content)
}
