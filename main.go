package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/playwright-community/playwright-go"
)

// containsArg checks if a specific flag is present in os.Args
func containsArg(flag string) bool {
	for _, arg := range os.Args {
		if arg == flag {
			return true
		}
	}
	return false
}

// getTargetURL returns the first non-flag argument (the job URL)
func getTargetURL() string {
	for _, arg := range os.Args[1:] {
		if arg != "--batch" && arg != "--dry-run" && arg != "--anonymize-logs" {
			return arg
		}
	}
	return ""
}

func main() {
	_ = godotenv.Load()
	db := InitDB("applications.db")
	defer db.Close()

	// Initialize encryption — loads key, auto-migrates plaintext secrets
	if err := InitEncryption(); err != nil {
		log.Fatalf("Encryption init failed: %v", err)
	}

	// ---- Utility Commands (no Playwright needed) ----
	if containsArg("--anonymize-logs") {
		if err := AnonymizeLogs("audit.log", "audit_anonymized.log"); err != nil {
			log.Fatalf("Anonymization failed: %v", err)
		}
		return
	}

	cvBytes, err := os.ReadFile("my_cv.txt")
	if err != nil {
		fmt.Println("⚠️  No 'my_cv.txt' found. Creating a template...")
		os.WriteFile("my_cv.txt", []byte("Name: \nLocation: \nExperience: \nSkills: \n"), 0644)
		os.Exit(1)
	}
	myCV := string(cvBytes)

	dryRun := containsArg("--dry-run")

	// Initialize Playwright ONCE for the entire application lifecycle!
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("Could not start Playwright: %v", err)
	}
	defer pw.Stop()

	// Launch single stealth Chromium instance for scraping
	browser, scrapePage, err := launchStealthBrowser(pw, true)
	if err != nil {
		log.Fatalf("Could not launch stealth browser: %v", err)
	}
	defer browser.Close()
	scrapePage.Close() // We only needed it for the init script; workers create their own pages

	if len(os.Args) > 1 {
		if containsArg("--batch") {
			RunBatch(pw, browser, db, "targets.txt", myCV)
			fmt.Println("\nPress Enter to open Dashboard...")
			fmt.Scanln()
		} else if targetURL := getTargetURL(); targetURL != "" {
			fmt.Printf("\n🚀 AGENT ACTIVATED\nTarget: %s\n", targetURL)
			if dryRun {
				fmt.Println("📋 [DRY RUN MODE] — No browser will open for form-filling.")
			}

			scrapedText, err := ScrapeJob(browser, targetURL)
			if err != nil {
				log.Fatalf("Scraper error: %v", err)
			}

			// Multi-CV: Pick the best CV for this job (if cvs/ exists)
			selectedCV := SelectBestCV(scrapedText, myCV)

			evaluation, err := EvaluateJob(scrapedText, selectedCV)
			if err != nil {
				log.Fatalf("Eval error: %v", err)
			}

			fmt.Printf("✅ Identified: %s - %s (Score: %.1f)\n", evaluation.Company, evaluation.Role, evaluation.Score)

			if evaluation.Score >= 4.0 {
				fmt.Println("✨ High Score! Generating tailored materials...")

				pdfName := fmt.Sprintf("Resume_%s.pdf", evaluation.Company)
				var finalCV *CVContent

				fmt.Println("   ✍️  [AI Writer] Drafting tailored CV... (This takes a few seconds)")
				draft, err := TailorCV(scrapedText, selectedCV, "")
				if err != nil {
					fmt.Printf("❌ Failed to tailor CV: %v\n", err)
				} else {
					// Best-draft tracking
					bestDraft := draft
					bestScore := 0

					fmt.Println("   🕵️  [AI Critic] Reviewing draft against Job Description...")
					review, _ := ReviewCV(scrapedText, draft)
					attempts := 1

					if review != nil {
						bestScore = review.Score
					}

					for review != nil && !review.Approved && attempts <= 3 {
						fmt.Printf("   ⚠️  Critic rejected draft for %s (Score: %d/10). Writer revising...\n", evaluation.Company, review.Score)
						fmt.Println("   ✍️  [AI Writer] Revising tailored CV...")
						draft, _ = TailorCV(scrapedText, selectedCV, review.Feedback)
						if draft != nil {
							fmt.Println("   🕵️  [AI Critic] Reviewing revised draft...")
							review, _ = ReviewCV(scrapedText, draft)
							if review != nil && review.Score > bestScore {
								bestScore = review.Score
								bestDraft = draft
							}
						}
						attempts++
					}

					if review != nil && review.Approved {
						fmt.Printf("   ✨ Critic APPROVED final draft for %s!\n", evaluation.Company)
						GeneratePDF(pw, draft, pdfName)
						finalCV = draft
					} else {
						fmt.Printf("   ⚠️  Critic did not fully approve %s. Using best-scored draft (score: %d).\n", evaluation.Company, bestScore)
						if bestDraft != nil {
							GeneratePDF(pw, bestDraft, pdfName)
							finalCV = bestDraft
						}
					}
				}

				fmt.Println("   💡 [AI Strategist] Drafting application answers...")
				answers, err := DraftApplicationAnswers(browser, scrapedText, selectedCV, targetURL)
				if err == nil {
					fmt.Println("\n📝 DRAFT ANSWERS:")
					fmt.Printf("   Why Us: %s\n", answers.WhyCompany)
					fmt.Printf("   Tech Challenge: %s\n", answers.TechChallenge)

					// --- Interactive Answer Review ---
					fmt.Println("\n   ── Answer Review ──────────────────────────────")
					reader := bufio.NewReader(os.Stdin)

					fmt.Print("   'Why Us' — [Enter] to accept, or type replacement: ")
					if replacement, _ := reader.ReadString('\n'); strings.TrimSpace(replacement) != "" {
						answers.WhyCompany = strings.TrimSpace(replacement)
						fmt.Println("   ✏️  Updated 'Why Us' answer.")
					}

					fmt.Print("   'Tech Challenge' — [Enter] to accept, or type replacement: ")
					if replacement, _ := reader.ReadString('\n'); strings.TrimSpace(replacement) != "" {
						answers.TechChallenge = strings.TrimSpace(replacement)
						fmt.Println("   ✏️  Updated 'Tech Challenge' answer.")
					}

					fmt.Println("   ── Answers confirmed ─────────────────────────")
				}

				// --- DRY RUN: Output JSON report instead of filling ---
				if dryRun {
					dryRunReport := map[string]interface{}{
						"mode":       "dry_run",
						"target_url": targetURL,
						"company":    evaluation.Company,
						"role":       evaluation.Role,
						"score":      evaluation.Score,
						"status":     evaluation.Status,
						"cv_data":    finalCV,
						"answers":    answers,
						"token_usage": map[string]int64{
							"estimated_used": GlobalBudget.EstimatedUsed,
							"max_tokens":     GlobalBudget.MaxTokens,
						},
					}
					reportJSON, _ := json.MarshalIndent(dryRunReport, "", "  ")
					reportFile := fmt.Sprintf("dry_run_%s.json", evaluation.Company)
					os.WriteFile(reportFile, reportJSON, 0600)
					fmt.Printf("\n📋 [DRY RUN] Report saved to: %s\n", reportFile)
					fmt.Println("   No browser was opened. No form was filled.")
				} else if finalCV != nil {
					// LIVE MODE: Auto-fill the form
					AutoFillApplication(pw, targetURL, finalCV, pdfName, answers)
				}
			}

			SaveApplication(db, evaluation.Company, evaluation.Role, evaluation.Score, evaluation.Status, targetURL)

			fmt.Printf("\n📊 Token Usage: ~%d / %d estimated tokens\n", GlobalBudget.EstimatedUsed, GlobalBudget.MaxTokens)
			fmt.Println("\nPress Enter to open Dashboard...")
			fmt.Scanln()
		}
	}

	p := tea.NewProgram(initialModel(db), tea.WithAltScreen())
	p.Run()
}
