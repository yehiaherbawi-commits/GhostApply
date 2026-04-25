package main

import (
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// ============================================================================
// SMART JOB DESCRIPTION EXTRACTION
// ============================================================================

// High-probability JD container selectors, ordered from most specific to least.
var jdSelectors = []string{
	// Workday
	"[data-automation-id='jobDescription']",
	"[data-automation-id='job-description']",
	// Common JD containers
	"div.job-description",
	"#jobDescription",
	"#job-description",
	"section.job-details",
	".jobDescriptionContent",
	".job-desc",
	"article.job-posting",
	"[class*='job-desc']",
	"[class*='jobDescription']",
	"[class*='job-description']",
	// Greenhouse, Lever, etc.
	"#content",
	".content-intro",
	".posting-page",
	// Broader fallbacks
	"main",
	"[role='main']",
	"article",
}

// boilerplatePatterns are substrings that indicate a line is noise, not JD content.
var boilerplatePatterns = []string{
	"cookie", "privacy policy", "terms of service", "terms of use",
	"all rights reserved", "©", "powered by", "subscribe to",
	"follow us", "connect with us", "social media", "newsletter",
	"accept all", "cookie settings", "manage consent",
}

// ExtractJobDescription navigates to a URL and extracts the job description text.
// It uses a cascading selector strategy:
//  1. Try high-probability JD container selectors
//  2. Check iframes recursively
//  3. Fallback to <body>
//  4. Clean boilerplate from the result
func ExtractJobDescription(browser playwright.Browser, url string) (string, error) {
	page, err := browser.NewPage()
	if err != nil {
		return "", fmt.Errorf("could not create page: %v", err)
	}
	defer page.Close()

	fmt.Printf("Navigating to: %s...\n", url)

	// Use networkidle with a timeout, fallback to domcontentloaded
	_, err = page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(15000), // 15s max for networkidle
	})
	if err != nil {
		// Fallback: retry with domcontentloaded (more lenient)
		fmt.Println("   ⚠️  Network idle timeout. Retrying with DOM content loaded...")
		_, err = page.Goto(url, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		})
		if err != nil {
			return "", fmt.Errorf("could not navigate to URL: %v", err)
		}
	}

	// Small delay for dynamic content to render
	humanDelay(1000, 2000)

	// Strategy 1: Try specific JD container selectors
	for _, sel := range jdSelectors {
		loc := page.Locator(sel).First()
		if count, _ := loc.Count(); count > 0 {
			if visible, _ := loc.IsVisible(); visible {
				text, err := loc.InnerText()
				if err == nil && len(strings.TrimSpace(text)) > 50 {
					fmt.Printf("   📋 [JD Extraction] Found container: %s\n", sel)
					return cleanBoilerplate(text), nil
				}
			}
		}
	}

	// Strategy 2: Check iframes recursively
	text := extractFromIframes(page)
	if text != "" {
		fmt.Println("   📋 [JD Extraction] Found content in iframe")
		return cleanBoilerplate(text), nil
	}

	// Strategy 3: Fallback to <body>
	fmt.Println("   ⚠️  [JD Extraction] No specific container found. Using <body> fallback.")
	bodyText, err := page.Locator("body").InnerText()
	if err != nil {
		return "", fmt.Errorf("could not extract text: %v", err)
	}

	return cleanBoilerplate(bodyText), nil
}

// extractFromIframes recursively checks all iframes for JD content
func extractFromIframes(page playwright.Page) string {
	frames := page.MainFrame().ChildFrames()

	for _, frame := range frames {
		// Try known JD selectors within each iframe
		for _, sel := range jdSelectors {
			loc := frame.Locator(sel).First()
			if count, _ := loc.Count(); count > 0 {
				text, err := loc.InnerText()
				if err == nil && len(strings.TrimSpace(text)) > 50 {
					return text
				}
			}
		}

		// Fallback: try body of the iframe
		bodyLoc := frame.Locator("body").First()
		if count, _ := bodyLoc.Count(); count > 0 {
			text, err := bodyLoc.InnerText()
			if err == nil && len(strings.TrimSpace(text)) > 100 {
				return text
			}
		}
	}

	return ""
}

// cleanBoilerplate removes common noise from extracted text:
// cookie banners, footer links, navigation items, and excessive whitespace.
func cleanBoilerplate(text string) string {
	lines := strings.Split(text, "\n")
	var cleaned []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Skip very short lines (likely nav items, breadcrumbs)
		if len(trimmed) < 3 {
			continue
		}

		// Skip boilerplate patterns
		lineLower := strings.ToLower(trimmed)
		isBoilerplate := false
		for _, pattern := range boilerplatePatterns {
			if strings.Contains(lineLower, pattern) {
				isBoilerplate = true
				break
			}
		}
		if isBoilerplate {
			continue
		}

		cleaned = append(cleaned, trimmed)
	}

	result := strings.Join(cleaned, "\n")

	// Trim to max 8000 chars to avoid token waste in AI calls
	if len(result) > 8000 {
		result = result[:8000] + "\n[... truncated for AI processing]"
	}

	return result
}

// ScrapeJob is a backward-compatible wrapper around ExtractJobDescription.
func ScrapeJob(browser playwright.Browser, url string) (string, error) {
	return ExtractJobDescription(browser, url)
}
