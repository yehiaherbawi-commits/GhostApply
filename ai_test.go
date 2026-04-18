package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Clean JSON with markdown backticks",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "Clean JSON with markdown backticks but no json keyword",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "Clean JSON with no backticks",
			input:    "{\"key\": \"value\"}",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "Clean JSON with leading/trailing whitespace",
			input:    "   \n\t```json\n{\"key\": \"value\"}\n```\n  ",
			expected: "{\"key\": \"value\"}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []genai.Part{
								genai.Text(tc.input),
							},
						},
					},
				},
			}
			actual := cleanJSONResponse(resp)
			if actual != tc.expected {
				t.Errorf("cleanJSONResponse() = %v, want %v", actual, tc.expected)
			}
		})
	}
}

type mockTransport struct {
	responseBody string
	statusCode   int
	err          error
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       ioutil.NopCloser(bytes.NewBufferString(m.responseBody)),
		Header:     make(http.Header),
	}, nil
}

func TestTailorCV_Success(t *testing.T) {
	os.Setenv("GEMINI_API_KEY", "fake")

	mockJSON := `{"name": "John Doe", "email": "john@example.com", "phone": "123", "linkedin": "linked.com", "location": "NY", "role": "Dev", "summary": "A dev", "experience": ["Did things"]}`
	apiResponse := `{"candidates":[{"content":{"parts":[{"text":"` + strings.ReplaceAll(mockJSON, `"`, `\"`) + `"}]}}]}`

	mockClient := &http.Client{
		Transport: &mockTransport{
			responseBody: apiResponse,
			statusCode:   200,
		},
	}

	genaiClientOptions = []option.ClientOption{option.WithHTTPClient(mockClient)}
	defer func() { genaiClientOptions = nil }()

	cv, err := TailorCV("Software Engineer", "My old CV", "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cv.Name != "John Doe" {
		t.Errorf("Expected Name 'John Doe', got '%s'", cv.Name)
	}
}

func TestTailorCV_JSONParseFailure(t *testing.T) {
	os.Setenv("GEMINI_API_KEY", "fake")

	// Return invalid JSON from the API
	apiResponse := `{"candidates":[{"content":{"parts":[{"text":"Invalid JSON response"}]}}]}`

	mockClient := &http.Client{
		Transport: &mockTransport{
			responseBody: apiResponse,
			statusCode:   200,
		},
	}

	genaiClientOptions = []option.ClientOption{option.WithHTTPClient(mockClient)}
	defer func() { genaiClientOptions = nil }()

	cv, err := TailorCV("Software Engineer", "My old CV", "")
	if err == nil {
		t.Fatalf("Expected error for invalid JSON, got none. CV: %v", cv)
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Errorf("Expected invalid character error, got: %v", err)
	}
}

func TestTailorCV_APIFailure(t *testing.T) {
	os.Setenv("GEMINI_API_KEY", "fake")

	mockClient := &http.Client{
		Transport: &mockTransport{
			responseBody: `{}`,
			statusCode:   500, // API Failure
		},
	}

	genaiClientOptions = []option.ClientOption{option.WithHTTPClient(mockClient)}
	defer func() { genaiClientOptions = nil }()

	cv, err := TailorCV("Software Engineer", "My old CV", "")
	if err == nil {
		t.Fatalf("Expected API error, got none. CV: %v", cv)
	}
	if !strings.Contains(err.Error(), "googleapi: Error 500") {
		t.Errorf("Expected 500 error, got: %v", err)
	}
}
