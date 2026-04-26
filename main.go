package main

import (
	"bufio"
	"database/sql"
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
		if arg != "--batch" && arg != "--dry-run" && arg != "--anonymize-logs" && arg != "--onboard" && arg != "--verify" && arg != "compare" && arg != "--crawl-list" {
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
	if containsArg("--onboard") {
		RunOnboarding()
		return
	}
	if containsArg("--verify") {
		VerifyPipeline(db)
		return
	}
	if containsArg("compare") {
		CompareOffers(db)
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

			if isJobListPage(targetURL, scrapedText) {
				if containsArg("--crawl-list") {
					fmt.Println("🔎 Listing page detected. Auto-crawling individual jobs...")
					crawlAndProcessJobs(pw, browser, targetURL, myCV, db)
					return
				} else {
					fmt.Println("❌ The provided URL appears to be a job listing page, not a single job posting.")
					fmt.Println("   Please use the direct URL to an individual job (e.g., .../ExternalJobDetail?id=...).")
					fmt.Println("   If you want GhostApply to process all jobs from this list, run with --crawl-list.")
					os.Exit(1)
				}
			}

			// Multi-CV: Pick the best CV for this job (if cvs/ exists)
			selectedCV := SelectBestCV(scrapedText, myCV)

			evaluation, err := EvaluateJob(scrapedText, selectedCV)
			if err != nil {
				log.Fatalf("Eval error: %v", err)
			}

			fmt.Printf("✅ Identified: %s - %s (Score: %.1f)\n", evaluation.Company, evaluation.Role, evaluation.Score)

			var reportPath string
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
						finalCV = draft
					} else {
						fmt.Printf("   ⚠️  Critic did not fully approve %s. Using best-scored draft (score: %d).\n", evaluation.Company, bestScore)
						if bestDraft != nil {
							finalCV = bestDraft
						}
					}
				}

				// ATS Keyword Injection
				if finalCV != nil {
					fmt.Println("   🎯 [ATS Injector] Optimizing CV with keywords from Job Description...")
					injectedCV, err := InjectATSKeywords(scrapedText, finalCV)
					if err == nil {
						finalCV = injectedCV
					}
					GeneratePDF(pw, finalCV, pdfName)
				}

				// Deep Evaluation & Story Bank
				fmt.Println("   📊 [AI Deep Eval] Performing Deep Evaluation...")
				deepReport, rPath, err := DeepEvaluateJob(browser, scrapedText, selectedCV, targetURL)
				if err == nil && deepReport != nil {
					reportPath = rPath
					fmt.Println("   📚 [Story Bank] Appending stories to Story Bank...")
					AppendStories(deepReport.InterviewPrep.Stories, evaluation.Company)
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

			SaveApplication(db, evaluation.Company, evaluation.Role, evaluation.Score, evaluation.Status, targetURL, reportPath)

			fmt.Printf("\n📊 Token Usage: ~%d / %d estimated tokens\n", GlobalBudget.EstimatedUsed, GlobalBudget.MaxTokens)
			fmt.Println("\nPress Enter to open Dashboard...")
			fmt.Scanln()
		}
	}

	p := tea.NewProgram(initialModel(db), tea.WithAltScreen())
	p.Run()
}

func crawlAndProcessJobs(pw *playwright.Playwright, browser playwright.Browser, listURL string, myCV string, db *sql.DB) {
	fmt.Println("   🕷️  Crawling listing page for job URLs...")
	
	page, err := browser.NewPage()
	if err != nil {
		fmt.Printf("❌ Failed to create page for crawling: %v\n", err)
		return
	}
	defer page.Close()

	_, err = page.Goto(listURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		fmt.Printf("❌ Failed to navigate to listing page: %v\n", err)
		return
	}

	links, err := page.Locator("a").All()
	if err != nil {
		fmt.Printf("❌ Failed to find links: %v\n", err)
		return
	}

	var jobURLs []string
	seen := make(map[string]bool)

	importNetURL := false // Just to avoid unused import errors if we did strings processing manually.
	_ = importNetURL

	// Actually, let's just do a naive prefix check instead of importing net/url to avoid adding more imports at the top
	// and potentially causing compilation errors if we don't manage the imports correctly via replace_file_content.
	for _, link := range links {
		href, err := link.GetAttribute("href")
		if err != nil || href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			continue
		}
		
		// Very naive base URL prepending for relative links
		if strings.HasPrefix(href, "/") {
			parts := strings.Split(listURL, "/")
			if len(parts) >= 3 {
				baseURL := parts[0] + "//" + parts[2]
				href = baseURL + href
			}
		}

		hrefLower := strings.ToLower(href)
		// SuccessFactors specific and general heuristics
		if strings.Contains(hrefLower, "job") || strings.Contains(hrefLower, "career") || strings.Contains(hrefLower, "position") || strings.Contains(hrefLower, "posting") || strings.Contains(hrefLower, "detail") || strings.Contains(hrefLower, "apply") {
			// Avoid the current listing page
			if hrefLower != strings.ToLower(listURL) && !seen[href] {
				seen[href] = true
				jobURLs = append(jobURLs, href)
			}
		}
	}

	if len(jobURLs) == 0 {
		fmt.Println("   ⚠️  Could not find any obvious job links on the page.")
		return
	}

	fmt.Printf("   ✅ Found %d potential job links. Saving to temp_targets.txt and starting batch process...\n", len(jobURLs))
	
	err = os.WriteFile("temp_targets.txt", []byte(strings.Join(jobURLs, "\n")), 0644)
	if err != nil {
		fmt.Printf("❌ Failed to create temp targets file: %v\n", err)
		return
	}
	defer os.Remove("temp_targets.txt")

	RunBatch(pw, browser, db, "temp_targets.txt", myCV)
}
