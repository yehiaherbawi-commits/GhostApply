package main

import (
	"fmt"

	"github.com/playwright-community/playwright-go"
)

// ScrapeJob boots up a headless browser, navigates to a URL, and grabs the text.
func ScrapeJob(url string) (string, error) {
	// 1. Start the Playwright engine
	pw, err := playwright.Run()
	if err != nil {
		return "", fmt.Errorf("could not start playwright: %v", err)
	}
	// Make sure we stop the engine when the function finishes
	defer pw.Stop()

	// 2. Launch Chromium in the background (headless mode)
	browser, err := pw.Chromium.Launch()
	if err != nil {
		return "", fmt.Errorf("could not launch browser: %v", err)
	}
	defer browser.Close()

	// 3. Open a fresh tab
	page, err := browser.NewPage()
	if err != nil {
		return "", fmt.Errorf("could not create page: %v", err)
	}

	// 4. Navigate to the job posting
	fmt.Printf("Navigating to: %s...\n", url)

	// We wait for 'Networkidle' which ensures all the dynamic ATS content has loaded
	_, err = page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		return "", fmt.Errorf("could not navigate to URL: %v", err)
	}

	// 5. Extract the raw text from the body of the page
	// InnerText strips away all the messy HTML tags and gives us clean reading material
	text, err := page.Locator("body").InnerText()
	if err != nil {
		return "", fmt.Errorf("could not extract text: %v", err)
	}

	return text, nil
}
