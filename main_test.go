// main_test.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shayyz-code/raccoon-sku/backend/llama"
)

// 1. Create a mock struct that satisfies the `asker` interface.
type mockAsker struct{}

// This is our mock Ask method. It doesn't make any network calls.
// It just returns a fixed response, allowing us to test the handler logic.
func (m *mockAsker) Ask(prompt llama.Prompt) (string, error) {
	if prompt.User == "make_error" {
		return "", fmt.Errorf("mock AI error")
	}
	return "mocked AI reply", nil
}

func TestHandleAskLlama_Success(t *testing.T) {
	// Create an application instance using our MOCK asker.
	app := &application{
		ai: &mockAsker{},
	}

	// Create a sample JSON request body.
	requestBody, _ := json.Marshal(map[string]string{
		"systemPrompt": "test system",
		"userPrompt":   "test user",
	})

	// Create a new HTTP request and a recorder to capture the response.
	req := httptest.NewRequest("POST", "/api/ask-llama", bytes.NewBuffer(requestBody))
	rr := httptest.NewRecorder()

	// Call the handler directly.
	app.handleAskLlama(rr, req)

	// Assert the HTTP status code.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Assert the response body.
	expected := `{"reply":"mocked AI reply"}` + "\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}