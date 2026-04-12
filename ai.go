package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type Evaluation struct {
	Company   string  `json:"company"`
	Role      string  `json:"role"`
	Score     float64 `json:"score"`
	Status    string  `json:"status"`
	Rationale string  `json:"rationale"`
}

type CVContent struct {
	Name       string   `json:"name"`
	Location   string   `json:"location"`
	Role       string   `json:"role"`
	Summary    string   `json:"summary"`
	Experience []string `json:"experience"`
}

// NEW: For those high-score applications
type AppAnswers struct {
	WhyCompany    string `json:"why_company"`
	TechChallenge string `json:"tech_challenge"`
}

// generateContentWithRetry wraps the Gemini API with Exponential Backoff and Jitter
func generateContentWithRetry(ctx context.Context, model *genai.GenerativeModel, prompt string) (*genai.GenerateContentResponse, error) {
	maxRetries := 3
	baseSleep := 60 * time.Second

	// Create a local random number generator for our Jitter
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := model.GenerateContent(ctx, genai.Text(prompt))
		if err == nil {
			return resp, nil // Success! Break the loop and return.
		}

		// Check if the error is related to 429 Quotas
		if strings.Contains(err.Error(), "429") || strings.Contains(strings.ToLower(err.Error()), "quota") {
			if attempt == maxRetries {
				return nil, fmt.Errorf("max retries (%d) reached after rate limits: %v", maxRetries, err)
			}

			// Calculate Jitter: Random 0-15 seconds + 10s escalation per retry
			jitter := time.Duration(rng.Intn(15)) * time.Second
			escalation := time.Duration(attempt*10) * time.Second
			sleepTime := baseSleep + escalation + jitter

			fmt.Printf("\n⏳ [Cooldown] Rate limit hit. Worker sleeping for %v...\n", sleepTime)
			time.Sleep(sleepTime)
			fmt.Println("🚀 [Cooldown] Worker resuming...")
			continue // Loop back and try the API call again
		}

		// If it's a different kind of error (like a network drop), abort immediately
		return nil, err
	}

	return nil, fmt.Errorf("unexpected end of retry loop")
}

func EvaluateJob(jobText string, myCV string) (*Evaluation, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	client, _ := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash")
	model.ResponseMIMEType = "application/json"

	systemInstruction := `You are an expert recruiter.
	Output a JSON object with the exact keys: 'company', 'role', 'score' (out of 5.0), 'status' ('Apply' or 'Skip'), and 'rationale'.`
	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}

	prompt := fmt.Sprintf("CV:\n%s\n\nJD:\n%s", myCV, jobText)
	resp, err := generateContentWithRetry(ctx, model, prompt)
	if err != nil {
		return nil, err
	}

	var eval Evaluation
	rawString := string(resp.Candidates[0].Content.Parts[0].(genai.Text))
	rawString = strings.TrimSpace(rawString)
	rawString = strings.TrimPrefix(rawString, "```json")
	rawString = strings.TrimPrefix(rawString, "```")
	rawString = strings.TrimSuffix(rawString, "```")
	rawString = strings.TrimSpace(rawString)

	err = json.Unmarshal([]byte(rawString), &eval)
	if err != nil {
		fmt.Printf("❌ JSON Parse Error: %v\nRAW LLM OUTPUT:\n%s\n", err, rawString)
	}

	return &eval, nil
}

func TailorCV(jobText string, myCV string) (*CVContent, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	client, _ := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash")
	model.ResponseMIMEType = "application/json"

	// THE UPGRADE: Added strict schema enforcement to prevent nested objects
	systemInstruction := `You are an ATS expert. Extract the candidate's 'name', 'location', and 'role' from their CV. 
	Write a 'summary' paragraph and rewrite their experience into an array of tailored bullets.
	
	CRITICAL OUTPUT RULES:
	Output a JSON object with EXACTLY these keys: 'name', 'location', 'role', 'summary', and 'experience'.
	The 'experience' key MUST be a simple, flat array of strings (e.g., ["Bullet 1", "Bullet 2"]). 
	DO NOT output nested objects, dictionaries, or sub-keys for the experience.`
	
	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}

	prompt := fmt.Sprintf("CV:\n%s\n\nJD:\n%s", myCV, jobText)
	resp, err := generateContentWithRetry(ctx, model, prompt)
	if err != nil {
		return nil, err
	}

	var cv CVContent
	rawString := string(resp.Candidates[0].Content.Parts[0].(genai.Text))
	rawString = strings.TrimSpace(rawString)
	rawString = strings.TrimPrefix(rawString, "```json")
	rawString = strings.TrimPrefix(rawString, "```")
	rawString = strings.TrimSuffix(rawString, "```")
	rawString = strings.TrimSpace(rawString)

	err = json.Unmarshal([]byte(rawString), &cv)
	if err != nil {
		fmt.Printf("❌ JSON Parse Error in TailorCV: %v\nRAW LLM OUTPUT:\n%s\n", err, rawString)
	}
	
	return &cv, nil
}

func DraftApplicationAnswers(jobText string, myCV string) (*AppAnswers, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	client, _ := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash")
	model.ResponseMIMEType = "application/json"

	systemInstruction := `Based on the CV and JD, write two professional answers for an application form:
	1. "Why do you want to work here?" (Focus on company mission and technical fit)
	2. "Describe a technical challenge." (Use a real project from the CV)
	Keep each answer under 150 words. Output JSON.`

	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}

	prompt := fmt.Sprintf("CV:\n%s\n\nJD:\n%s", myCV, jobText)
	resp, err := generateContentWithRetry(ctx, model, prompt)
	if err != nil {
		return nil, err
	}

	var answers AppAnswers
	rawString := string(resp.Candidates[0].Content.Parts[0].(genai.Text))
	rawString = strings.TrimSpace(rawString)
	rawString = strings.TrimPrefix(rawString, "```json")
	rawString = strings.TrimPrefix(rawString, "```")
	rawString = strings.TrimSuffix(rawString, "```")
	rawString = strings.TrimSpace(rawString)

	err = json.Unmarshal([]byte(rawString), &answers)
	if err != nil {
		fmt.Printf("❌ JSON Parse Error in DraftAnswers: %v\nRAW LLM OUTPUT:\n%s\n", err, rawString)
	}

	return &answers, nil
}
