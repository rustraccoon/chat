// llama/llama_test.go
package llama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAsk_Success tests the happy path where the API returns a valid response.
func TestAsk_Success(t *testing.T) {
	// 1. Create a mock server that simulates the Groq API.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert that the request method is POST
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Assert that the Authorization header is set correctly
		expectedAuth := "Bearer FAKE_API_KEY"
		if r.Header.Get("Authorization") != expectedAuth {
			t.Errorf("Expected Authorization header '%s', got '%s'", expectedAuth, r.Header.Get("Authorization"))
		}

		// Send a predefined successful response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		mockResponse := GroqResponse{
			Choices: []Choice{
				{
					Message:      &GroqMessage{Role: "assistant", Content: "Mingalaba! Surprise, I'm a testing bot!"},
					FinishReason: "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	// 2. Set environment variables for the test to use our mock server.
	// t.Setenv is the modern way to handle this (Go 1.17+)
	t.Setenv("GROQ_API_URL", server.URL)
	t.Setenv("GROQ_API_KEY", "FAKE_API_KEY")
	t.Setenv("GROQ_MODEL", "test-model")

	// 3. Call the function we are testing.
	prompt := Prompt{System: "test system", User: "test user"}
	reply, err := LlamaService.Ask(LlamaService{},prompt)

	// 4. Assert the results.
	if err != nil {
		t.Fatalf("Ask() returned an unexpected error: %v", err)
	}

	expectedReply := "Mingalaba! Surprise, I'm a testing bot!"
	if reply != expectedReply {
		t.Errorf("Expected reply '%s', got '%s'", expectedReply, reply)
	}
}

// TestAsk_ApiError tests how our function handles an error from the API.
func TestAsk_ApiError(t *testing.T) {
	// 1. Create a mock server that returns an error status.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Groq Error"))
	}))
	defer server.Close()

	// 2. Set environment variables for the test.
	t.Setenv("GROQ_API_URL", server.URL)
	t.Setenv("GROQ_API_KEY", "FAKE_API_KEY")
	t.Setenv("GROQ_MODEL", "test-model")

	// 3. Call the function.
	prompt := Prompt{System: "test system", User: "test user"}
	_, err := LlamaService.Ask(LlamaService{}, prompt)

	// 4. Assert that we received an error.
	if err == nil {
		t.Fatal("Ask() was expected to return an error, but it did not")
	}

	expectedErrorMsg := "groq API error: status code 500, body: Internal Groq Error"
	if err.Error() != expectedErrorMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, err.Error())
	}
}