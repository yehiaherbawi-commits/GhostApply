package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/playwright-community/playwright-go"
)

// UPGRADED: Channel payload now carries the final approved CV
type JobResult struct {
	URL        string
	Eval       *Evaluation
	TailoredCV *CVContent
	Error      error
}

func worker(id int, jobs <-chan string, results chan<- JobResult, wg *sync.WaitGroup, cv string, browser playwright.Browser) {
	defer wg.Done()

	for url := range jobs {
		fmt.Printf("⚡ [Worker %d] Processing: %s\n", id, url)

		text, err := ScrapeJob(browser, url)
		if err != nil {
			results <- JobResult{URL: url, Error: fmt.Errorf("scrape failed: %v", err)}
			continue
		}

		eval, err := EvaluateJob(text, cv)
		if err != nil {
			results <- JobResult{URL: url, Error: fmt.Errorf("eval failed: %v", err)}
			continue
		}

		var finalCV *CVContent

		// MULTI-AGENT LOOP
		if eval.Score >= 4.0 {
			fmt.Printf("   ✍️  [Worker %d] AI Writer drafting tailored CV...\n", id)
			draft, _ := TailorCV(text, cv, "")
			if draft != nil {
				fmt.Printf("   🕵️  [Worker %d] AI Critic reviewing draft...\n", id)
				review, _ := ReviewCV(text, draft)
				attempts := 1

				// If the Critic rejects it, loop and force a rewrite! (Max 3 rewrites)
				for review != nil && !review.Approved && attempts <= 3 {
					fmt.Printf("   ⚠️  [Worker %d] Critic rejected draft for %s (Score: %d/10). Writer revising...\n", id, eval.Company, review.Score)
					fmt.Printf("   ✍️  [Worker %d] AI Writer revising CV...\n", id)
					draft, _ = TailorCV(text, cv, review.Feedback)
					if draft != nil {
						fmt.Printf("   🕵️  [Worker %d] AI Critic reviewing revised draft...\n", id)
						review, _ = ReviewCV(text, draft)
					}
					attempts++
				}

				if review != nil && review.Approved {
					fmt.Printf("   ✨ [Worker %d] Critic APPROVED final draft for %s!\n", id, eval.Company)
					finalCV = draft
				} else {
					fmt.Printf("   ⚠️  [Worker %d] Critic did not approve %s. Using best-effort draft.\n", id, eval.Company)
					finalCV = draft
				}
			}
		}

		results <- JobResult{URL: url, Eval: eval, TailoredCV: finalCV}
	}
}

func RunBatch(pw *playwright.Playwright, browser playwright.Browser, db *sql.DB, filePath string, cv string) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url != "" && !strings.HasPrefix(url, "#") {
			urls = append(urls, url)
		}
	}

	fmt.Printf("\n--- 🌀 STARTING MULTI-AGENT FACTORY (%d URLs) ---\n", len(urls))

	jobs := make(chan string, len(urls))
	results := make(chan JobResult, len(urls))

	var wg sync.WaitGroup
	var dbWg sync.WaitGroup

	// The Database Listener (Safe & Fast)
	dbWg.Add(1)
	go func() {
		defer dbWg.Done()
		for res := range results {
			if res.Error != nil {
				fmt.Printf("⚠️  [Failed] %s: %v\n", res.URL, res.Error)
				continue
			}

			SaveApplication(db, res.Eval.Company, res.Eval.Role, res.Eval.Score, res.Eval.Status, res.URL)
			fmt.Printf("✅ [Logged] %s - %s (Score: %.1f)\n", res.Eval.Company, res.Eval.Role, res.Eval.Score)

			// Generate the PDF using the pre-approved CV from the worker!
			if res.TailoredCV != nil {
				safeName := strings.ReplaceAll(res.Eval.Company, " ", "_")
				GeneratePDF(pw, res.TailoredCV, fmt.Sprintf("Resume_%s.pdf", safeName))
			}
		}
	}()

	numWorkers := 5
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg, cv, browser)
	}

	for _, url := range urls {
		jobs <- url
	}
	close(jobs)

	wg.Wait()
	close(results)
	dbWg.Wait()

	fmt.Println("\n--- ✅ BATCH COMPLETE ---")
}
