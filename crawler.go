package main

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// isNavigationURL returns true if the URL is a known non-job link.
func isNavigationURL(rawURL string) bool {
	excludePatterns := []string{
		"/Login", "/Error", "/Jobs?",
		"/SearchJobs", "/careers/", "/how-we-hire",
		"/early-career", "labor-condition", "service-now.com",
		"/extjobs", "facebook", "twitter", "linkedin", "youtube",
		"/privacy", "/terms", "/contact", "/sitemap",
	}
	lower := strings.ToLower(rawURL)
	for _, pat := range excludePatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// isJobPostingURL returns true if the URL looks like an individual job posting.
func isJobPostingURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := parsed.Path
	query := parsed.Query()
	// SuccessFactors: FolderDetail with folderId is actually the listing page or individual?
	// Wait, the user said FolderDetail with folderId is what we want to INCLUDE? 
	// The prompt says: "if strings.Contains(path, "/FolderDetail") && query.Get("folderId") != "" { return true }"
	if strings.Contains(path, "/FolderDetail") && query.Get("folderId") != "" {
		return true
	}
	// SuccessFactors alternative: ExternalJobDetail
	if strings.Contains(path, "/ExternalJobDetail") {
		return true
	}
	// Generic job posting patterns
	if strings.Contains(path, "/job/") || strings.Contains(path, "/jobs/") {
		if query.Get("jobId") != "" || query.Get("id") != "" {
			return true
		}
	}
	// Broaden a bit for other ATS
	if strings.Contains(path, "/position/") || strings.Contains(path, "/posting/") {
		return true
	}
	return false
}

// resolveURL makes relative URLs absolute
func resolveURL(base, href string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return href
	}
	hrefURL, err := url.Parse(href)
	if err != nil {
		return href
	}
	return baseURL.ResolveReference(hrefURL).String()
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

	// Give JS a bit of time to render the list
	humanDelay(2000, 3000)

	// Attempt to restrict search to the main content area to avoid header/footer nav
	container := page.Locator("main, [role='main'], #content, .content").First()
	if count, _ := container.Count(); count == 0 {
		container = page.Locator("body")
	}

	links, err := container.Locator("a").All()
	if err != nil {
		fmt.Printf("❌ Failed to find links: %v\n", err)
		return
	}

	var jobURLs []string
	seen := make(map[string]bool)

	for _, link := range links {
		href, err := link.GetAttribute("href")
		if err != nil || href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			continue
		}

		absoluteURL := resolveURL(listURL, href)

		if isNavigationURL(absoluteURL) || !isJobPostingURL(absoluteURL) {
			continue
		}

		// Avoid the current listing page
		if strings.EqualFold(absoluteURL, listURL) {
			continue
		}

		if !seen[absoluteURL] {
			seen[absoluteURL] = true

			// Additional check: is this link inside a job card (not footer/nav)?
			parentText, _ := link.Evaluate("el => el.closest('li, div, tr')?.innerText || ''", nil)
			parentLower := strings.ToLower(fmt.Sprint(parentText))
			
			// If it's a known generic list, we might miss some, but the user explicitly requested this check
			if strings.Contains(parentLower, "job id:") || strings.Contains(parentLower, "experience level:") || strings.Contains(parentLower, "posted") {
				jobURLs = append(jobURLs, absoluteURL)
			} else {
				// Fallback: If we can't find specific markers, but it's clearly a job link based on URL and not nav,
				// we could include it. But the user asked to restrict it. Let's just append it if we are fairly certain.
				// We'll trust the URL heuristics if the parent text is empty (e.g. hidden).
				if parentLower == "" {
					jobURLs = append(jobURLs, absoluteURL)
				}
			}
		}
	}

	if len(jobURLs) == 0 {
		fmt.Println("   ⚠️  Could not find any obvious job links on the page.")
		return
	}

	fmt.Printf("   ✅ Found %d unique job links. Saving to temp_targets.txt and starting batch process...\n", len(jobURLs))

	err = os.WriteFile("temp_targets.txt", []byte(strings.Join(jobURLs, "\n")), 0644)
	if err != nil {
		fmt.Printf("❌ Failed to create temp targets file: %v\n", err)
		return
	}
	defer os.Remove("temp_targets.txt")

	RunBatch(pw, browser, db, "temp_targets.txt", myCV)
}
