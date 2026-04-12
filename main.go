package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	db := InitDB("applications.db")
	defer db.Close()

	cvBytes, err := os.ReadFile("my_cv.txt")
	if err != nil {
		fmt.Println("⚠️  No 'my_cv.txt' found. Creating a template...")
		templateCV := `Name: Your Name
Location: Your City, Country
Experience: 5 years as a Software Engineer.
Education: BSc Computer Science
Skills: Go, Python, SQL, REST APIs.
`
		os.WriteFile("my_cv.txt", []byte(templateCV), 0644)
		fmt.Println("ℹ️  Please open 'my_cv.txt', fill in your actual resume details, and run the agent again.")
		os.Exit(1)
	}
	myCV := string(cvBytes)

	if len(os.Args) > 1 {
		arg := os.Args[1]

		// --- NEW: Route to the Goroutine factory if --batch is passed ---
		if arg == "--batch" {
			RunBatch(db, "targets.txt", myCV)

			fmt.Println("\nPress Enter to open Dashboard...")
			fmt.Scanln()
		} else {
			// --- EXISTING: Single URL Logic ---
			targetURL := arg
			fmt.Printf("\n🚀 AGENT ACTIVATED\nTarget: %s\n", targetURL)

			scrapedText, err := ScrapeJob(targetURL)
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

				// Your safe error-handling block preserved perfectly
				tailored, err := TailorCV(scrapedText, myCV)
				if err != nil {
					fmt.Printf("❌ Failed to tailor CV (Gemini Error): %v\n", err)
				} else {
					GeneratePDF(tailored, fmt.Sprintf("Resume_%s.pdf", evaluation.Company))
				}

				answers, err := DraftApplicationAnswers(scrapedText, myCV)
				if err != nil {
					fmt.Printf("❌ Failed to draft answers (Gemini Error): %v\n", err)
				} else {
					fmt.Println("\n📝 DRAFT ANSWERS:")
					fmt.Printf("Why Us: %s\n", answers.WhyCompany)
				}
			}

			SaveApplication(db, evaluation.Company, evaluation.Role, evaluation.Score, evaluation.Status, targetURL)

			fmt.Println("\nPress Enter to open Dashboard...")
			fmt.Scanln()
		}
	}

	p := tea.NewProgram(initialModel(db), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
