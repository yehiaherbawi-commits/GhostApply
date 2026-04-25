package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/playwright-community/playwright-go"
)

// ============================================================================
// STEALTH BROWSER — Anti-Bot Detection Countermeasures
// ============================================================================

// Realistic Chrome user agent string (Chrome 124 on Windows 10)
const stealthUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// stealthInitScript is injected into every page to hide automation signals
const stealthInitScript = `
// Delete the webdriver property that Playwright sets
Object.defineProperty(navigator, 'webdriver', {
    get: () => undefined,
});

// Override navigator.plugins to return a non-empty array (real browsers have plugins)
Object.defineProperty(navigator, 'plugins', {
    get: () => [
        { name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer' },
        { name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai' },
        { name: 'Native Client', filename: 'internal-nacl-plugin' },
    ],
});

// Override navigator.languages for consistency
Object.defineProperty(navigator, 'languages', {
    get: () => ['en-US', 'en', 'de'],
});

// Hide the automation-related Chrome properties
if (window.chrome === undefined) {
    window.chrome = {};
}
window.chrome.runtime = window.chrome.runtime || {};

// Prevent detection via permissions API
const originalQuery = window.navigator.permissions.query;
window.navigator.permissions.query = (parameters) => (
    parameters.name === 'notifications' ?
        Promise.resolve({ state: Notification.permission }) :
        originalQuery(parameters)
);
`

// launchStealthBrowser launches a Chromium browser with anti-detection measures.
// headless controls whether the browser window is visible.
// Returns the browser and a pre-configured page.
func launchStealthBrowser(pw *playwright.Playwright, headless bool) (playwright.Browser, playwright.Page, error) {
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-infobars",
			"--no-first-run",
			"--no-default-browser-check",
			"--disable-extensions",
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not launch stealth browser: %v", err)
	}

	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		UserAgent: playwright.String(stealthUserAgent),
		Viewport: &playwright.Size{
			Width:  1920,
			Height: 1080,
		},
		Locale: playwright.String("en-US"),
	})
	if err != nil {
		browser.Close()
		return nil, nil, fmt.Errorf("could not create stealth page: %v", err)
	}

	// Inject stealth script that runs before any page scripts
	if err := page.AddInitScript(playwright.Script{Content: playwright.String(stealthInitScript)}); err != nil {
		fmt.Printf("   ⚠️  Failed to inject stealth script: %v\n", err)
	}

	return browser, page, nil
}

// ============================================================================
// HUMAN-LIKE DELAYS — Randomized Pauses with Normal Distribution
// ============================================================================

// humanDelay pauses for a random duration between minMs and maxMs milliseconds.
// The distribution is centered around the midpoint with a bell curve shape,
// making most pauses close to the average while allowing occasional outliers.
func humanDelay(minMs, maxMs int) {
	if minMs >= maxMs {
		time.Sleep(time.Duration(minMs) * time.Millisecond)
		return
	}

	mid := float64(minMs+maxMs) / 2.0
	stddev := float64(maxMs-minMs) / 6.0 // 99.7% within range

	// Box-Muller transform for normal distribution
	u1 := rand.Float64()
	u2 := rand.Float64()
	z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)

	delay := mid + z*stddev

	// Clamp to bounds
	if delay < float64(minMs) {
		delay = float64(minMs)
	}
	if delay > float64(maxMs) {
		delay = float64(maxMs)
	}

	time.Sleep(time.Duration(delay) * time.Millisecond)
}

// humanType types text character by character with random delays between
// keystrokes, mimicking real human typing speed (50-150ms per character).
func humanType(page playwright.Page, selector string, text string) bool {
	loc := page.Locator(selector).First()
	count, _ := loc.Count()
	if count == 0 {
		return false
	}

	// Click the field first
	if err := loc.Click(); err != nil {
		return false
	}

	// Clear existing content
	page.Keyboard().Press("Control+a")
	humanDelay(50, 100)
	page.Keyboard().Press("Backspace")
	humanDelay(100, 200)

	// Type character by character
	for _, ch := range text {
		page.Keyboard().Type(string(ch))
		humanDelay(50, 150) // Realistic keystroke interval
	}

	return true
}
