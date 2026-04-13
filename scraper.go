package main

import (
	"fmt"

	"github.com/playwright-community/playwright-go"
)

// ScrapeJob boots up a headless browser, navigates to a URL, and grabs the text.
func ScrapeJob(browser playwright.Browser, url string) (string, error) {
	// 3. Open a fresh tab
	page, err := browser.NewPage()
	if err != nil {
		return "", fmt.Errorf("could not create page: %v", err)
	}

	// 4. Navigate to the job posting
	fmt.Printf("Navigating to: %s...\n", url)

	// We wait for 'domcontentloaded' or 'load' instead of 'networkidle' to prevent hanging
	// on pages with constant polling or WebSockets.
	_, err = page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
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
