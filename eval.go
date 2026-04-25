package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/playwright-community/playwright-go"
	"google.golang.org/api/option"
)

type STARStory struct {
	Requirement string `json:"requirement"`
	Situation   string `json:"situation"`
	Task        string `json:"task"`
	Action      string `json:"action"`
	Result      string `json:"result"`
	Reflection  string `json:"reflection"`
}

type DeepReport struct {
	RoleSummary struct {
		Archetype string `json:"archetype"`
		Domain    string `json:"domain"`
		Seniority string `json:"seniority"`
		Remote    string `json:"remote"`
		Company   string `json:"company"`
		Title     string `json:"title"`
		TLDR      string `json:"tldr"`
	} `json:"role_summary"`
	CVMatch []struct {
		Requirement string `json:"requirement"`
		CVLine      string `json:"cv_line"` // The matching text from the candidate's CV
		Mitigation  string `json:"mitigation"` // If missing, how to mitigate it
	} `json:"cv_match"`
	LevelStrategy struct {
		DetectedLevel string `json:"detected_level"`
		SellSenior    string `json:"sell_senior"`
		Negotiation   string `json:"negotiation"`
	} `json:"level_strategy"`
	Compensation struct {
		Score   int      `json:"score"`
		Details string   `json:"details"`
		Sources []string `json:"sources"`
	} `json:"compensation"`
	Personalisation struct {
		CVChanges       []string `json:"cv_changes"`
		LinkedInChanges []string `json:"linkedin_changes"`
	} `json:"personalisation"`
	InterviewPrep struct {
		Stories   []STARStory `json:"stories"`
		CaseStudy string      `json:"case_study"`
		RedFlags  []string    `json:"red_flags"`
	} `json:"interview_prep"`
	GlobalScore float64 `json:"global_score"` // 0.0 to 5.0
}

func DeepEvaluateJob(browser playwright.Browser, jobText string, cvText string, targetURL string) (*DeepReport, string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()

	opts := []option.ClientOption{option.WithAPIKey(apiKey)}
	opts = append(opts, genaiClientOptions...)
	client, err := genai.NewClient(ctx, opts...)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-pro")
	model.ResponseMIMEType = "application/json"

	systemInstruction := `You are an elite Career-Ops AI Agent performing a Deep Evaluation of a Job Description against a Candidate's CV.
You must output a JSON object representing the DeepReport structure.
- RoleSummary.Archetype must be one of: LLMOps, Agentic Automation, AI PM, AI Solutions Architect, Forward Deployed Engineer, AI Transformation Lead, or Software Engineer.
- CVMatch: Map every major JD requirement to a specific line/word from the candidate's CV. If the candidate lacks it, provide a concrete mitigation strategy (e.g., adjacent experience, portfolio project). If they have it, set mitigation to empty string.
- LevelStrategy: Compare detected JD seniority vs candidate's natural level. Provide a "sell senior without lying" script and a "if down-levelled, negotiation" script.
- Compensation: Provide realistic salary estimates and a comp score (1-5).
- Personalisation: 5 concrete CV changes (bullet reordering, keyword injection, summary rewrite) and 5 LinkedIn changes.
- InterviewPrep: Provide 6-10 STAR+R (Situation, Task, Action, Result, Reflection) stories mapped to JD requirements. Suggest 1 case study. List red-flag questions.
- GlobalScore: A float from 0.0 to 5.0 weighted on CV match, North-Star alignment, comp, and cultural signals.
IMPORTANT: Respond in the language of the Job Description.`
	
	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}

	prompt := fmt.Sprintf("CANDIDATE CV:\n%s\n\nJOB DESCRIPTION:\n%s", cvText, jobText)
	
	fmt.Println("   🔍 [Deep Eval] Running extensive analysis with Gemini...")
	resp, err := generateContentWithRetry(ctx, model, prompt)
	if err != nil {
		return nil, "", fmt.Errorf("gemini generation failed: %w", err)
	}

	var report DeepReport
	rawString := cleanJSONResponse(resp)
	err = json.Unmarshal([]byte(rawString), &report)
	if err != nil {
		return nil, "", fmt.Errorf("json unmarshal failed: %w\nRAW: %s", err, rawString)
	}

	// Legitmacy Check
	legitimacyStatus := "High Confidence"
	if browser != nil {
		fmt.Println("   🔍 [Deep Eval] Checking posting legitimacy via Playwright...")
		page, err := browser.NewPage()
		if err == nil {
			defer page.Close()
			resp, err := page.Goto(targetURL, playwright.PageGotoOptions{
				WaitUntil: playwright.WaitUntilStateDomcontentloaded,
				Timeout:   playwright.Float(15000),
			})
			if err != nil || resp == nil || !resp.Ok() {
				legitimacyStatus = "Suspicious (Failed to load or 404)"
			} else {
				content, _ := page.Content()
				contentLower := strings.ToLower(content)
				if strings.Contains(contentLower, "position filled") || 
				   strings.Contains(contentLower, "no longer accepting applications") ||
				   strings.Contains(contentLower, "job is closed") {
					legitimacyStatus = "Proceed with Caution (Possible closed/filled)"
				}
			}
		} else {
			legitimacyStatus = "Unknown (Browser error)"
		}
	}

	// Generate Markdown and save
	reportPath, err := saveDeepReport(&report, targetURL, legitimacyStatus)
	if err != nil {
		fmt.Printf("   ⚠️  [Deep Eval] Could not save report: %v\n", err)
	}

	return &report, reportPath, nil
}

func saveDeepReport(r *DeepReport, url string, legitimacy string) (string, error) {
	err := os.MkdirAll("reports", 0755)
	if err != nil {
		return "", err
	}

	timestamp := time.Now().Format("2006-01-02")
	id := time.Now().Unix() % 10000 // simple uniqueish ID
	safeCompany := strings.ReplaceAll(strings.ToLower(r.RoleSummary.Company), " ", "_")
	if safeCompany == "" {
		safeCompany = "unknown"
	}
	filename := fmt.Sprintf("reports/%04d-%s-%s.md", id, safeCompany, timestamp)

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Deep Evaluation Report: %s at %s\n\n", r.RoleSummary.Title, r.RoleSummary.Company))
	sb.WriteString(fmt.Sprintf("**Global Match Score:** %.1f / 5.0\n", r.GlobalScore))
	sb.WriteString(fmt.Sprintf("**URL:** %s\n", url))
	sb.WriteString(fmt.Sprintf("**Legitimacy Status:** %s\n\n", legitimacy))
	sb.WriteString("---\n\n")

	sb.WriteString("## Block A – Role Summary\n")
	sb.WriteString(fmt.Sprintf("- **Archetype:** %s\n", r.RoleSummary.Archetype))
	sb.WriteString(fmt.Sprintf("- **Domain:** %s\n", r.RoleSummary.Domain))
	sb.WriteString(fmt.Sprintf("- **Seniority:** %s\n", r.RoleSummary.Seniority))
	sb.WriteString(fmt.Sprintf("- **Remote Policy:** %s\n", r.RoleSummary.Remote))
	sb.WriteString(fmt.Sprintf("\n> **TL;DR:** %s\n\n", r.RoleSummary.TLDR))

	sb.WriteString("## Block B – CV Match Analysis\n")
	sb.WriteString("| Requirement | CV Match | Mitigation Strategy |\n")
	sb.WriteString("|---|---|---|\n")
	for _, m := range r.CVMatch {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", m.Requirement, m.CVLine, m.Mitigation))
	}
	sb.WriteString("\n")

	sb.WriteString("## Block C – Level Strategy\n")
	sb.WriteString(fmt.Sprintf("- **Detected Level:** %s\n", r.LevelStrategy.DetectedLevel))
	sb.WriteString(fmt.Sprintf("- **Sell Senior Script:** %s\n", r.LevelStrategy.SellSenior))
	sb.WriteString(fmt.Sprintf("- **Negotiation Script:** %s\n\n", r.LevelStrategy.Negotiation))

	sb.WriteString("## Block D – Compensation Research\n")
	sb.WriteString(fmt.Sprintf("- **Score:** %d / 5\n", r.Compensation.Score))
	sb.WriteString(fmt.Sprintf("- **Details:** %s\n", r.Compensation.Details))
	sb.WriteString("- **Sources:** " + strings.Join(r.Compensation.Sources, ", ") + "\n\n")

	sb.WriteString("## Block E – Personalisation Plan\n")
	sb.WriteString("### CV Changes\n")
	for _, c := range r.Personalisation.CVChanges {
		sb.WriteString(fmt.Sprintf("- %s\n", c))
	}
	sb.WriteString("### LinkedIn Changes\n")
	for _, c := range r.Personalisation.LinkedInChanges {
		sb.WriteString(fmt.Sprintf("- %s\n", c))
	}
	sb.WriteString("\n")

	sb.WriteString("## Block F – Interview Preparation\n")
	sb.WriteString(fmt.Sprintf("- **Case Study to Present:** %s\n\n", r.InterviewPrep.CaseStudy))
	sb.WriteString("### Red Flags to Ask About\n")
	for _, rf := range r.InterviewPrep.RedFlags {
		sb.WriteString(fmt.Sprintf("- %s\n", rf))
	}
	sb.WriteString("\n### STAR+R Stories\n")
	for i, s := range r.InterviewPrep.Stories {
		sb.WriteString(fmt.Sprintf("#### Story %d: %s\n", i+1, s.Requirement))
		sb.WriteString(fmt.Sprintf("- **Situation:** %s\n", s.Situation))
		sb.WriteString(fmt.Sprintf("- **Task:** %s\n", s.Task))
		sb.WriteString(fmt.Sprintf("- **Action:** %s\n", s.Action))
		sb.WriteString(fmt.Sprintf("- **Result:** %s\n", s.Result))
		sb.WriteString(fmt.Sprintf("- **Reflection:** %s\n\n", s.Reflection))
	}

	err = os.WriteFile(filename, []byte(sb.String()), 0644)
	if err != nil {
		return "", err
	}
	return filename, nil
}
