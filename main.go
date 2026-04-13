package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/playwright-community/playwright-go"
)

func main() {
	_ = godotenv.Load()
	db := InitDB("applications.db")
	defer db.Close()

	cvBytes, err := os.ReadFile("my_cv.txt")
	if err != nil {
		fmt.Println("⚠️  No 'my_cv.txt' found. Creating a template...")
		os.WriteFile("my_cv.txt", []byte("Name: \nLocation: \nExperience: \nSkills: \n"), 0644)
		os.Exit(1)
	}
	myCV := string(cvBytes)

	// Initialize Playwright ONCE for the entire application lifecycle!
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("Could not start Playwright: %v", err)
	}
	defer pw.Stop()

	// Launch single Chromium instance for scraping
	browser, err := pw.Chromium.Launch()
	if err != nil {
		log.Fatalf("Could not launch Chromium: %v", err)
	}
	defer browser.Close()

	if len(os.Args) > 1 {
		arg := os.Args[1]

		if arg == "--batch" {
			RunBatch(pw, browser, db, "targets.txt", myCV)
			fmt.Println("\nPress Enter to open Dashboard...")
			fmt.Scanln()
		} else {
			targetURL := arg
			fmt.Printf("\n🚀 AGENT ACTIVATED\nTarget: %s\n", targetURL)

			scrapedText, err := ScrapeJob(browser, targetURL)
			if err != nil {
				log.Fatalf("Scraper error: %v", err)
			}

			evaluation, err := EvaluateJob(scrapedText, myCV)
			if err != nil {
				log.Fatalf("Eval error: %v", err)
			}

			fmt.Printf("✅ Identified: %s - %s (Score: %.1f)\n", evaluation.Company, evaluation.Role, evaluation.Score)

			if evaluation.Score >= 4.0 {
				fmt.Println("✨ High Score! Generating tailored materials...")

				pdfName := fmt.Sprintf("Resume_%s.pdf", evaluation.Company)
				var finalCV *CVContent

				fmt.Println("   ✍️  [AI Writer] Drafting tailored CV... (This takes a few seconds)")
				draft, err := TailorCV(scrapedText, myCV, "")
				if err != nil {
					fmt.Printf("❌ Failed to tailor CV: %v\n", err)
				} else {
					fmt.Println("   🕵️  [AI Critic] Reviewing draft against Job Description...")
					review, _ := ReviewCV(scrapedText, draft)
					attempts := 1

					for review != nil && !review.Approved && attempts <= 2 {
						fmt.Printf("   ⚠️  Critic rejected draft for %s (Score: %d/10). Writer revising...\n", evaluation.Company, review.Score)
						fmt.Println("   ✍️  [AI Writer] Revising tailored CV...")
						draft, _ = TailorCV(scrapedText, myCV, review.Feedback)
						if draft != nil {
							fmt.Println("   🕵️  [AI Critic] Reviewing revised draft...")
							review, _ = ReviewCV(scrapedText, draft)
						}
						attempts++
					}

					if review != nil && review.Approved {
						fmt.Printf("   ✨ Critic APPROVED final draft for %s!\n", evaluation.Company)
						GeneratePDF(pw, draft, pdfName)
						finalCV = draft
					} else {
						fmt.Printf("   ❌ Critic gave up on %s. Draft not approved (No PDF generated).\n", evaluation.Company)
					}
				}

				fmt.Println("   💡 [AI Strategist] Drafting application answers...")
				answers, err := DraftApplicationAnswers(browser, scrapedText, myCV, targetURL)
				if err == nil {
					fmt.Printf("\n📝 DRAFT ANSWERS:\nWhy Us: %s\n", answers.WhyCompany)
				}

				// --- NEW: THE FINAL BOSS ---
				// If we successfully tailored the CV and generated a PDF, auto-fill the form!
				if finalCV != nil {
					AutoFillApplication(pw, targetURL, finalCV, pdfName, answers)
				}
			}

			SaveApplication(db, evaluation.Company, evaluation.Role, evaluation.Score, evaluation.Status, targetURL)

			fmt.Println("\nPress Enter to open Dashboard...")
			fmt.Scanln()
		}
	}

	p := tea.NewProgram(initialModel(db), tea.WithAltScreen())
	p.Run()
}
