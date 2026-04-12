package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
)

// JobResult bundles everything together so the workers can send it down the channel
type JobResult struct {
	URL     string
	Eval    *Evaluation
	RawText string // We keep the raw text so we don't have to scrape twice for the PDF
	Error   error
}

// The Worker function runs concurrently, grabbing URLs from the jobs channel
func worker(id int, jobs <-chan string, results chan<- JobResult, wg *sync.WaitGroup, cv string) {
	defer wg.Done()

	for url := range jobs {
		fmt.Printf("⚡ [Worker %d] Processing: %s\n", id, url)

		// 1. Scrape
		text, err := ScrapeJob(url)
		if err != nil {
			results <- JobResult{URL: url, Error: fmt.Errorf("scrape failed: %v", err)}
			continue
		}

		// 2. Evaluate
		eval, err := EvaluateJob(text, cv)
		if err != nil {
			results <- JobResult{URL: url, Error: fmt.Errorf("eval failed: %v", err)}
			continue
		}

		// 3. Send Success Result to the channel
		results <- JobResult{URL: url, Eval: eval, RawText: text}
	}
}

// RunBatch is the main dispatcher
func RunBatch(db *sql.DB, filePath string, cv string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("❌ Could not open batch file: %v\n", err)
		return
	}
	defer file.Close()

	// Read all URLs into a list
	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url != "" && !strings.HasPrefix(url, "#") { // Ignore empty lines and comments
			urls = append(urls, url)
		}
	}

	if len(urls) == 0 {
		fmt.Println("⚠️ No URLs found in targets.txt")
		return
	}

	fmt.Printf("\n--- 🌀 STARTING BATCH FACTORY (%d URLs) ---\n", len(urls))

	// 1. Create our communication channels
	jobs := make(chan string, len(urls))
	results := make(chan JobResult, len(urls))

	var wg sync.WaitGroup
	var dbWg sync.WaitGroup

	// 2. Start the Database Listener (Only ONE of these to prevent SQLite locking!)
	dbWg.Add(1)
	go func() {
		defer dbWg.Done()
		for res := range results {
			if res.Error != nil {
				fmt.Printf("⚠️  [Failed] %s: %v\n", res.URL, res.Error)
				continue
			}

			// Save to SQLite
			SaveApplication(db, res.Eval.Company, res.Eval.Role, res.Eval.Score, res.Eval.Status, res.URL)
			fmt.Printf("✅ [Logged] %s - %s (Score: %.1f)\n", res.Eval.Company, res.Eval.Role, res.Eval.Score)

			// Generate PDF if it's a high score
			if res.Eval.Score >= 4.0 {
				tailored, err := TailorCV(res.RawText, cv)
				if err == nil {
					// Clean up the company name for the file name
					safeName := strings.ReplaceAll(res.Eval.Company, " ", "_")
					pdfName := fmt.Sprintf("Resume_%s.pdf", safeName)
					GeneratePDF(tailored, pdfName)
				}
			}
		}
	}()

	// 3. Spin up the Worker Pool (5 Concurrent AI Agents)
	numWorkers := 5
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg, cv)
	}

	// 4. Dispatch the jobs into the channel
	for _, url := range urls {
		jobs <- url
	}
	close(jobs) // Tell the workers no more jobs are coming

	// 5. Wait for all workers to finish their current tasks
	wg.Wait()
	close(results) // Tell the Database Listener no more results are coming

	// 6. Wait for the Database Listener to finish saving the last few items
	dbWg.Wait()

	fmt.Println("\n--- ✅ BATCH COMPLETE ---")
}
