package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"

	"github.com/google/generative-ai-go/genai"
	"github.com/playwright-community/playwright-go"
	"google.golang.org/api/option"
)

// InjectATSKeywords extracts 15-20 keywords from the JD and injects them into the CV summary and experience
func InjectATSKeywords(jobText string, cv *CVContent) (*CVContent, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()

	opts := []option.ClientOption{option.WithAPIKey(apiKey)}
	opts = append(opts, genaiClientOptions...)
	client, err := genai.NewClient(ctx, opts...)
	if err != nil {
		return cv, err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-pro")
	model.ResponseMIMEType = "application/json"

	systemInstruction := `You are an expert ATS Optimization Agent.
Your task is to:
1. Extract 15-20 critical keywords/skills from the Job Description.
2. Rewrite the candidate's Professional Summary to naturally inject as many of these keywords as possible.
3. Reorder the Experience bullets so that the most relevant ones to the JD appear first. You may slightly rephrase them to include keywords, but DO NOT invent skills the candidate does not have.
IMPORTANT: The generated summary and experience text MUST be written in the same language as the Job Description (e.g., if the JD is in German, output German text).
Return the complete CV JSON matching the exact original structure, but with the updated 'summary' and 'experience' arrays. Keep the exact same fields: 'name', 'email', 'phone', 'linkedin', 'location', 'role', 'summary', 'experience'.`

	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}

	cvJSON, _ := json.Marshal(cv)
	prompt := fmt.Sprintf("CANDIDATE CV:\n%s\n\nJOB DESCRIPTION:\n%s", string(cvJSON), jobText)

	fmt.Println("   🎯 [ATS Injector] Optimizing CV with keywords from Job Description...")
	resp, err := generateContentWithRetry(ctx, model, prompt)
	if err != nil {
		return cv, err
	}

	var newCV CVContent
	rawString := cleanJSONResponse(resp)
	err = json.Unmarshal([]byte(rawString), &newCV)
	if err != nil {
		return cv, err
	}

	return &newCV, nil
}

// GeneratePDF takes the tailored content and saves it as a PDF file
func GeneratePDF(pw *playwright.Playwright, content *CVContent, filename string) error {
	browser, _ := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	defer browser.Close()

	page, _ := browser.NewPage()

	tmpl, err := template.ParseFiles("templates/cv-template.html")
	if err != nil {
		return fmt.Errorf("could not parse cv-template.html: %v", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, content)
	if err != nil {
		return fmt.Errorf("could not execute template: %v", err)
	}

	// Set the content of the page
	page.SetContent(buf.String())

	// Print to PDF
	_, err = page.PDF(playwright.PagePdfOptions{
		Path:            playwright.String(filename),
		Format:          playwright.String("A4"),
		PrintBackground: playwright.Bool(true),
	})

	if err != nil {
		return fmt.Errorf("failed to create PDF: %v", err)
	}

	fmt.Printf("   📄 PDF Resume generated: %s\n", filename)
	return nil
}
