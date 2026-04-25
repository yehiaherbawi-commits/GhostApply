package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// UpdateMarkdownTracker appends a new entry to the applications.md file
func UpdateMarkdownTracker(app Application) {
	dir := "data"
	os.MkdirAll(dir, 0755)
	filename := filepath.Join(dir, "applications.md")

	var sb strings.Builder

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		sb.WriteString("# JobAgent Pipeline Tracker\n\n")
		sb.WriteString("| ID | Date | Company | Role | Score | Status | Report |\n")
		sb.WriteString("|---|---|---|---|---|---|---|\n")
	}

	reportLink := "-"
	if app.ReportPath != "" {
		reportLink = fmt.Sprintf("[View](%s)", app.ReportPath)
	}

	sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %.1f | %s | %s |\n",
		app.ID, app.Date, app.Company, app.Role, app.Score, app.Status, reportLink))

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("⚠️  Could not update %s: %v\n", filename, err)
		return
	}
	defer f.Close()

	f.WriteString(sb.String())
}

// VerifyPipeline checks the database against the filesystem to ensure consistency
func VerifyPipeline(db *sql.DB) {
	fmt.Println("\n🔍 [Pipeline Verifier] Checking system integrity...")

	apps, err := GetApplications(db)
	if err != nil {
		fmt.Printf("❌ Database error: %v\n", err)
		return
	}

	errorsFound := 0

	for _, app := range apps {
		if app.Score >= 4.0 {
			if app.ReportPath == "" {
				fmt.Printf("   ⚠️  [ID: %d] %s has a high score (%.1f) but no report path is logged.\n", app.ID, app.Company, app.Score)
				errorsFound++
			} else {
				if _, err := os.Stat(app.ReportPath); os.IsNotExist(err) {
					fmt.Printf("   ❌ [ID: %d] %s report file is missing: %s\n", app.ID, app.Company, app.ReportPath)
					errorsFound++
				}
			}
		}
	}

	if errorsFound == 0 {
		fmt.Println("   ✅ Pipeline integrity is PERFECT! All high-score jobs have reports.")
	} else {
		fmt.Printf("   ⚠️  Found %d issues in the pipeline.\n", errorsFound)
	}
}

type offerScore struct {
	company string
	role    string
	score   float64
}

// CompareOffers reads the reports and ranks them based on Global Score
func CompareOffers(db *sql.DB) {
	apps, err := GetApplications(db)
	if err != nil || len(apps) == 0 {
		fmt.Println("❌ No applications to compare.")
		return
	}

	fmt.Println("\n🏆 [Offer Comparison] Ranking highest matched jobs:")
	
	var offers []offerScore
	for _, app := range apps {
		if app.Score >= 4.0 && app.ReportPath != "" {
			offers = append(offers, offerScore{company: app.Company, role: app.Role, score: app.Score})
		}
	}

	sort.Slice(offers, func(i, j int) bool {
		return offers[i].score > offers[j].score
	})

	for i, o := range offers {
		fmt.Printf("   %d. %s - %s (Score: %.1f)\n", i+1, o.company, o.role, o.score)
	}

	if len(offers) == 0 {
		fmt.Println("   No deep evaluation reports found to compare.")
	}
}
