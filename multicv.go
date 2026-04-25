package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// ============================================================================
// MULTI-CV SUPPORT — AI-Driven Resume Selection
// ============================================================================

// SelectBestCV checks if a `cvs/` directory exists and, if so, uses Gemini AI
// to pick the best-matching CV for a given job description. Falls back to
// my_cv.txt if the directory doesn't exist.
func SelectBestCV(jobDescription string, defaultCV string) string {
	cvDir := "cvs"

	// Check if cvs/ directory exists
	info, err := os.Stat(cvDir)
	if err != nil || !info.IsDir() {
		return defaultCV // No cvs/ directory — use default
	}

	// List all .txt files in cvs/
	entries, err := os.ReadDir(cvDir)
	if err != nil {
		fmt.Printf("   ⚠️  Could not read cvs/ directory: %v\n", err)
		return defaultCV
	}

	var cvFiles []string
	var cvContents []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		path := filepath.Join(cvDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		cvFiles = append(cvFiles, entry.Name())
		// Truncate CV content for the prompt to avoid token waste
		content := string(data)
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}
		cvContents = append(cvContents, content)
	}

	if len(cvFiles) == 0 {
		fmt.Println("   ⚠️  cvs/ directory is empty. Using default CV.")
		return defaultCV
	}

	if len(cvFiles) == 1 {
		// Only one CV — use it directly
		fullPath := filepath.Join(cvDir, cvFiles[0])
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return defaultCV
		}
		fmt.Printf("   📄 Only one CV found: %s\n", cvFiles[0])
		return string(data)
	}

	// Multiple CVs — ask AI to pick the best one
	fmt.Printf("   📁 Found %d CVs in cvs/ directory. Asking AI to pick the best match...\n", len(cvFiles))

	bestCV := pickBestCVWithAI(jobDescription, cvFiles, cvContents)
	if bestCV == "" {
		fmt.Println("   ⚠️  AI could not pick a CV. Using default.")
		return defaultCV
	}

	fullPath := filepath.Join(cvDir, bestCV)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		fmt.Printf("   ⚠️  Could not read selected CV %s: %v\n", bestCV, err)
		return defaultCV
	}

	fmt.Printf("   ✨ AI selected: %s\n", bestCV)
	return string(data)
}

// pickBestCVWithAI uses Gemini to compare CV summaries against a JD and return
// the filename of the best match.
func pickBestCVWithAI(jd string, filenames []string, contents []string) string {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return ""
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash")
	model.SetTemperature(0.1)

	// Build the prompt
	var sb strings.Builder
	sb.WriteString("You are an expert recruiter. Given the following job description and multiple CVs, ")
	sb.WriteString("pick the BEST matching CV. Return ONLY the filename, nothing else.\n\n")
	sb.WriteString("JOB DESCRIPTION:\n")
	if len(jd) > 3000 {
		jd = jd[:3000]
	}
	sb.WriteString(jd)
	sb.WriteString("\n\nAVAILABLE CVs:\n")

	for i, name := range filenames {
		sb.WriteString(fmt.Sprintf("\n--- %s ---\n", name))
		sb.WriteString(contents[i])
		sb.WriteString("\n")
	}

	sb.WriteString("\nReturn ONLY the filename (e.g., 'software_engineer.txt'). No explanation.")

	resp, err := generateContentWithRetry(ctx, model, sb.String())
	if err != nil {
		return ""
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		result := strings.TrimSpace(string(resp.Candidates[0].Content.Parts[0].(genai.Text)))
		// Validate the result is actually one of our filenames
		for _, name := range filenames {
			if strings.EqualFold(result, name) {
				return name
			}
		}
		// Try partial match (AI might have returned without extension)
		for _, name := range filenames {
			if strings.Contains(strings.ToLower(name), strings.ToLower(result)) {
				return name
			}
		}
	}

	return ""
}
