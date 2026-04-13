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
	"github.com/playwright-community/playwright-go"
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
	Email      string   `json:"email"`
	Phone      string   `json:"phone"`
	LinkedIn   string   `json:"linkedin"`
	Location   string   `json:"location"`
	Role       string   `json:"role"`
	Summary    string   `json:"summary"`
	Experience []string `json:"experience"`
}

type AppAnswers struct {
	WhyCompany    string `json:"why_company"`
	TechChallenge string `json:"tech_challenge"`
}

// NEW: The Critic's Evaluation Struct
type CriticReview struct {
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
	Approved bool   `json:"approved"`
}

func generateContentWithRetry(ctx context.Context, model *genai.GenerativeModel, prompt string) (*genai.GenerateContentResponse, error) {
	maxRetries := 3
	baseSleep := 60 * time.Second
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := model.GenerateContent(ctx, genai.Text(prompt))
		if err == nil {
			return resp, nil
		}

		if strings.Contains(err.Error(), "429") || strings.Contains(strings.ToLower(err.Error()), "quota") {
			if attempt == maxRetries {
				return nil, fmt.Errorf("max retries (%d) reached: %v", maxRetries, err)
			}
			jitter := time.Duration(rng.Intn(15)) * time.Second
			escalation := time.Duration(attempt*10) * time.Second
			sleepTime := baseSleep + escalation + jitter

			fmt.Printf("\n⏳ [Cooldown] Rate limit hit. Worker sleeping for %v...\n", sleepTime)
			time.Sleep(sleepTime)
			fmt.Println("🚀 [Cooldown] Worker resuming...")
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("unexpected end of retry loop")
}

func cleanJSONResponse(resp *genai.GenerateContentResponse) string {
	rawString := string(resp.Candidates[0].Content.Parts[0].(genai.Text))
	rawString = strings.TrimSpace(rawString)
	rawString = strings.TrimPrefix(rawString, "```json")
	rawString = strings.TrimPrefix(rawString, "```")
	rawString = strings.TrimSuffix(rawString, "```")
	return strings.TrimSpace(rawString)
}

func EvaluateJob(jobText string, myCV string) (*Evaluation, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	client, _ := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-pro")
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
	rawString := cleanJSONResponse(resp)
	err = json.Unmarshal([]byte(rawString), &eval)
	return &eval, err
}

// UPGRADED: Now accepts a feedback string from the Critic
func TailorCV(jobText string, myCV string, previousFeedback string) (*CVContent, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	client, _ := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-pro")
	model.ResponseMIMEType = "application/json"

	systemInstruction := `You are an ATS expert. Extract 'name', 'email', 'phone', 'linkedin', 'location', and 'role' from the CV. 
	Write a 'summary' paragraph and rewrite experience into an array of tailored bullets.
	CRITICAL: The 'experience' key MUST be a simple, flat array of strings (e.g., ["Bullet 1", "Bullet 2"]). DO NOT output nested objects.`
	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}

	prompt := fmt.Sprintf("CV:\n%s\n\nJD:\n%s", myCV, jobText)

	// If the Critic sent it back, append the feedback!
	if previousFeedback != "" {
		prompt += fmt.Sprintf("\n\nCRITIC FEEDBACK ON YOUR LAST DRAFT (Fix these issues):\n%s", previousFeedback)
	}

	resp, err := generateContentWithRetry(ctx, model, prompt)
	if err != nil {
		return nil, err
	}

	var cv CVContent
	rawString := cleanJSONResponse(resp)
	err = json.Unmarshal([]byte(rawString), &cv)
	return &cv, err
}

// NEW: The Critic Agent
func ReviewCV(jobText string, draftCV *CVContent) (*CriticReview, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	client, _ := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-pro")
	model.ResponseMIMEType = "application/json"

	systemInstruction := `You are a strict but fair Hiring Manager reviewing an ATS-optimized resume draft against a Job Description.
	Look for hallucinated skills (skills the candidate does NOT have in their original CV), weak action verbs, or missed keyword opportunities.
	IMPORTANT: Do NOT penalize the candidate for skills they genuinely lack. Only penalize if the Writer fabricated skills not present in the original CV.
	Output a JSON object with EXACTLY these keys:
	- 'score': (Integer 0 to 10)
	- 'feedback': (A string detailing exactly what the writer must fix)
	- 'approved': (Boolean. True if score is 6 or higher).`
	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}

	draftJSON, _ := json.Marshal(draftCV)
	prompt := fmt.Sprintf("JOB DESCRIPTION:\n%s\n\nRESUME DRAFT:\n%s", jobText, string(draftJSON))

	resp, err := generateContentWithRetry(ctx, model, prompt)
	if err != nil {
		return nil, err
	}

	var review CriticReview
	rawString := cleanJSONResponse(resp)
	err = json.Unmarshal([]byte(rawString), &review)
	return &review, err
}

func DraftApplicationAnswers(browser playwright.Browser, jobText string, myCV string, targetURL string) (*AppAnswers, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	client, _ := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-pro")
	model.ResponseMIMEType = "application/json"

	// 1. Give the AI "Hands" (Define the Tool)
	scrapeTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "scrape_company_website",
			Description: "Scrape text from a URL. Use this to research the company's homepage or 'About Us' page to find recent facts before writing.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"url": {Type: genai.TypeString, Description: "The URL to scrape (e.g., https://fraunhofer.de)"},
				},
				Required: []string{"url"},
			},
		}},
	}
	model.Tools = []*genai.Tool{scrapeTool}

	systemInstruction := `You are an expert candidate drafting application answers.
	First, look at the JOB URL and infer the company's base website.
	You MUST use the 'scrape_company_website' tool to read their homepage or about page.
	Once you have the research, output a JSON object with EXACTLY these keys:
	- "why_company": Reference a specific fact/mission you learned from scraping their site.
	- "tech_challenge": Describe a technical challenge from the CV.`
	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}

	// 2. We use a Chat Session so the AI can talk back and forth with our Go code
	session := model.StartChat()
	prompt := fmt.Sprintf("JOB URL: %s\n\nCV:\n%s\n\nJD:\n%s", targetURL, myCV, jobText)

	// Turn 1: Send the prompt. The AI will reply with a request to run our tool.
	resp, err := session.SendMessage(ctx, genai.Text(prompt))
	if err != nil { return nil, err }

	// 3. The Interception: Check if the AI wants to use the tool
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		if funcCall, ok := resp.Candidates[0].Content.Parts[0].(genai.FunctionCall); ok {
			if funcCall.Name == "scrape_company_website" {
				urlToScrape := funcCall.Args["url"].(string)
				fmt.Printf("\n   🔍 [Researcher Agent] Autonomously navigating to: %s\n", urlToScrape)

				// Execute YOUR Go function on behalf of the AI!
				scrapedData, err := ScrapeJob(browser, urlToScrape)
				if err != nil {
					scrapedData = "Could not scrape. Just use the Job Description."
				}

				// Turn 2: Send the scraped website text back to the AI
				toolResult := map[string]any{"content": scrapedData}
				resp, err = session.SendMessage(ctx, genai.FunctionResponse{
					Name:     "scrape_company_website",
					Response: toolResult,
				})
				if err != nil { return nil, err }
			}
		}
	}

	// 4. Parse the final JSON response
	var answers AppAnswers
	rawString := cleanJSONResponse(resp)
	err = json.Unmarshal([]byte(rawString), &answers)
	if err != nil {
		fmt.Printf("❌ JSON Parse Error in DraftAnswers: %v\nRAW: %s\n", err, rawString)
	}

	return &answers, err
}

type AIAction struct {
	Type     string `json:"type"`     // "fill", "select", "click", "unknown_required"
	Selector string `json:"selector"` // CSS selector or Playwright selector
	Value    string `json:"value"`    // value to type/select
	Label    string `json:"label"`    // human-readable label
}

type AIActionPlan struct {
	Actions []AIAction `json:"actions"`
}

// AIFinishApplication uses Gemini to read ATS forms and inject profile data
func AIFinishApplication(formHTML string, profileJSON string) (*AIActionPlan, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	client, _ := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-pro")
	model.ResponseMIMEType = "application/json"

	systemInstruction := `You are an intelligent browser automation agent evaluating an ATS application form.
You are provided with the target HTML form and the candidate's JSON profile.
Your job is to identify all empty inputs, dropdowns, or selects in the form that correspond to the candidate's profile.
IGNORE standard name/email/phone fields if they look already filled or are standard, focus on subjective questions like "Availability", "Salary", "Location", "Visa", etc.

Return a JSON object with an 'actions' array.
For each relevant missing field:
- If you find an answer in the Candidate Profile, return an action.
- "type" should be "fill" (for text inputs) or "select" (for dropdowns <select>). Use "click" if it's a custom div-based dropdown option.
- "selector" MUST BE a valid Playwright selector (e.g., "input[name='available_from']" or "select#location_dropdown" or "label:has-text('Where are you currently located?') + select").
- "value" is the exact value to type or select. For "select", use the exact string option value or label.
- If a field is explicitly marked REQUIRED in the HTML (using 'required' attr or an asterisk '*') AND it's entirely missing from the candidate's profile, return an action with type "unknown_required" and "label" set to the field's name.`
	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}

	prompt := fmt.Sprintf("CANDIDATE PROFILE:\n%s\n\nFORM HTML:\n%s", profileJSON, formHTML)
	resp, err := generateContentWithRetry(ctx, model, prompt)
	if err != nil {
		return nil, err
	}

	var plan AIActionPlan
	rawString := cleanJSONResponse(resp)
	err = json.Unmarshal([]byte(rawString), &plan)
	if err != nil {
		fmt.Printf("❌ JSON Parse Error in AIFinishApplication: %v\nRAW: %s\n", err, rawString)
	}

	return &plan, err
}
