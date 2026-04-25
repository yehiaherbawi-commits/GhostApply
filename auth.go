package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/playwright-community/playwright-go"
	"golang.org/x/term"
)

// ============================================================================
// CREDENTIAL VAULT — Encrypted Login Credential Store (AES-256-GCM)
// ============================================================================

const (
	credentialsPlainFile = "credentials.json" // Legacy plaintext (auto-migrated)
	credentialsEncFile   = "credentials.enc"  // Encrypted vault
	encryptionKeyFile    = ".jobagent.key"     // Local encryption key
)

// encryptionKey holds the AES-256 key loaded at startup.
// It is set by InitEncryption() called from main().
var encryptionKey []byte

// InitEncryption loads (or creates) the encryption key and auto-migrates
// any plaintext secret files to their encrypted equivalents.
func InitEncryption() error {
	key, err := LoadOrCreateKey(encryptionKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load encryption key: %v", err)
	}
	encryptionKey = key

	// Auto-migrate plaintext files → encrypted
	if err := MigrateToEncrypted(key, credentialsPlainFile, credentialsEncFile); err != nil {
		fmt.Printf("   ⚠️  Credential migration warning: %v\n", err)
	}
	if err := MigrateToEncrypted(key, qaMemoryPlainFile, qaMemoryEncFile); err != nil {
		fmt.Printf("   ⚠️  QA Memory migration warning: %v\n", err)
	}

	return nil
}

type domainCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loadCredentials reads the encrypted credential vault from disk. Creates it if missing.
func loadCredentials() map[string]domainCredentials {
	creds := make(map[string]domainCredentials)

	found, err := ReadEncryptedJSON(encryptionKey, credentialsEncFile, &creds)
	if err != nil {
		fmt.Printf("   ⚠️  Credential vault error: %v — starting fresh\n", err)
		creds = make(map[string]domainCredentials)
		saveCredentials(creds)
	}
	if !found {
		saveCredentials(creds)
	}

	return creds
}

// saveCredentials encrypts and persists credentials to disk immediately.
func saveCredentials(creds map[string]domainCredentials) {
	if err := WriteEncryptedJSON(encryptionKey, credentialsEncFile, creds); err != nil {
		fmt.Printf("   ⚠️  Failed to save credentials: %v\n", err)
	}
}

// extractBaseDomain returns the base domain from a URL (e.g., "myworkdayjobs.com")
func extractBaseDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}

	host := parsed.Hostname()

	// Remove "www." prefix
	host = strings.TrimPrefix(host, "www.")

	// For subdomains like "company.myworkdayjobs.com", keep the last 2 parts
	// But for simple domains like "personio.com", keep as-is
	parts := strings.Split(host, ".")
	if len(parts) > 2 {
		// Keep the full host so "enopai.jobs.personio.com" stays distinct
		return host
	}

	return host
}

// askHumanForEmail prompts the user for an email (visible input)
func askHumanForEmail(domain string) string {
	fmt.Printf("   📧 Enter your Email for [%s]: ", domain)
	reader := bufio.NewReader(os.Stdin)
	email, _ := reader.ReadString('\n')
	return strings.TrimSpace(email)
}

// askHumanForPassword prompts the user for a password (hidden input using x/term)
func askHumanForPassword(domain string) string {
	fmt.Printf("   🔑 Enter your Password for [%s] (hidden): ", domain)
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Print newline after hidden input
	if err != nil {
		// Fallback to visible input if terminal doesn't support hidden mode
		fmt.Print("   🔑 Password (visible fallback): ")
		reader := bufio.NewReader(os.Stdin)
		pass, _ := reader.ReadString('\n')
		return strings.TrimSpace(pass)
	}
	return strings.TrimSpace(string(passwordBytes))
}

// ============================================================================
// AUTH WALL DETECTION & BYPASS
// ============================================================================

// detectAuthWall checks if the current page is a login/sign-in screen
func detectAuthWall(page playwright.Page) bool {
	// Primary signal: a password field exists
	pwField := page.Locator("input[type='password']")
	if count, _ := pwField.Count(); count > 0 {
		return true
	}

	// Secondary signals: login-specific headings or buttons
	loginSignals := []string{
		"h1:has-text('Sign In')", "h1:has-text('Log In')", "h1:has-text('Login')",
		"h2:has-text('Sign In')", "h2:has-text('Log In')", "h2:has-text('Login')",
		"button:has-text('Sign In')", "button:has-text('Log In')",
		"[data-automation-id='signInLink']", // Workday-specific
	}

	for _, selector := range loginSignals {
		loc := page.Locator(selector)
		if count, _ := loc.Count(); count > 0 {
			return true
		}
	}

	return false
}

// ============================================================================
// CAPTCHA DETECTION & MANUAL SOLVE
// ============================================================================

// detectCaptcha checks for common CAPTCHA patterns on the page
func detectCaptcha(page playwright.Page) bool {
	captchaSelectors := []string{
		// reCAPTCHA
		"iframe[src*='recaptcha']",
		"iframe[src*='google.com/recaptcha']",
		".g-recaptcha",
		"[data-sitekey]",
		// hCaptcha
		"iframe[src*='hcaptcha']",
		".h-captcha",
		// Generic CAPTCHA
		"iframe[src*='captcha']",
		"#captcha",
		"[class*='captcha' i]",
		"img[alt*='captcha' i]",
	}

	for _, sel := range captchaSelectors {
		loc := page.Locator(sel)
		if count, _ := loc.Count(); count > 0 {
			return true
		}
	}

	return false
}

// waitForCaptchaSolution pauses execution and instructs the user to solve
// the CAPTCHA manually in the visible browser window.
// timeoutSeconds: max wait time before aborting (default 120s).
func waitForCaptchaSolution(page playwright.Page, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}

	fmt.Println("\n   ┌─────────────────────────────────────────────────")
	fmt.Println("   │ 🤖 CAPTCHA DETECTED!")
	fmt.Println("   │ Please solve the CAPTCHA in the browser window.")
	fmt.Printf("   │ ⏰ Timeout: %d seconds\n", timeoutSeconds)
	fmt.Println("   └─────────────────────────────────────────────────")

	elapsed := 0
	for elapsed < timeoutSeconds {
		humanDelay(2000, 3000) // Poll every 2-3 seconds
		elapsed += 3

		if !detectCaptcha(page) {
			fmt.Println("   ✅ CAPTCHA solved! Continuing...")
			return nil
		}

		if elapsed%15 == 0 {
			fmt.Printf("   ⏳ Still waiting for CAPTCHA solution... (%ds/%ds)\n", elapsed, timeoutSeconds)
		}
	}

	return fmt.Errorf("CAPTCHA solve timeout after %d seconds", timeoutSeconds)
}

// ============================================================================
// MFA (Multi-Factor Authentication) DETECTION
// ============================================================================

// detectMFA checks if the current page is showing an MFA/2FA prompt
func detectMFA(page playwright.Page) bool {
	mfaSelectors := []string{
		// OTP/code input fields
		"input[name*='otp' i]",
		"input[name*='code' i]",
		"input[name*='token' i]",
		"input[name*='mfa' i]",
		"input[name*='verification' i]",
		"input[autocomplete='one-time-code']",
	}

	for _, sel := range mfaSelectors {
		loc := page.Locator(sel)
		if count, _ := loc.Count(); count > 0 {
			return true
		}
	}

	// Text-based signals
	mfaTextSignals := []string{
		"verification code", "two-factor", "2fa", "otp",
		"authenticator", "verify your identity", "enter the code",
		"security code", "bestätigungscode", "zwei-faktor",
	}

	bodyText, err := page.Locator("body").InnerText()
	if err == nil {
		bodyLower := strings.ToLower(bodyText)
		for _, signal := range mfaTextSignals {
			if strings.Contains(bodyLower, signal) {
				return true
			}
		}
	}

	return false
}

// waitForMFA pauses execution and instructs the user to complete the
// MFA challenge manually in the visible browser window.
func waitForMFA(page playwright.Page, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 180
	}

	fmt.Println("\n   ┌─────────────────────────────────────────────────")
	fmt.Println("   │ 🔐 MFA / TWO-FACTOR AUTHENTICATION DETECTED!")
	fmt.Println("   │ Please complete the verification in the browser.")
	fmt.Printf("   │ ⏰ Timeout: %d seconds\n", timeoutSeconds)
	fmt.Println("   └─────────────────────────────────────────────────")

	preURL := page.URL()
	elapsed := 0
	for elapsed < timeoutSeconds {
		humanDelay(3000, 4000) // Poll every 3-4 seconds
		elapsed += 4

		// MFA is complete if we navigated away or MFA elements disappeared
		if page.URL() != preURL || !detectMFA(page) {
			fmt.Println("   ✅ MFA completed! Continuing...")
			return nil
		}

		if elapsed%20 == 0 {
			fmt.Printf("   ⏳ Still waiting for MFA completion... (%ds/%ds)\n", elapsed, timeoutSeconds)
		}
	}

	return fmt.Errorf("MFA timeout after %d seconds", timeoutSeconds)
}

// handleAuthWall attempts to log in using stored or prompted credentials.
// Returns true if login was attempted, false if no auth wall was detected.
func handleAuthWall(page playwright.Page, jobURL string) bool {
	if !detectAuthWall(page) {
		return false
	}

	currentURL := page.URL()
	domain := extractBaseDomain(currentURL)
	fmt.Printf("\n🔒 [Auth Wall] Login required for domain: %s\n", domain)

	// ----- Load Credential Vault -----
	creds := loadCredentials()
	var email, password string

	if saved, ok := creds[domain]; ok {
		// Credentials found in vault
		fmt.Printf("   🔓 Found saved credentials for [%s]\n", domain)
		email = saved.Email
		password = saved.Password
	} else {
		// No credentials — ask the human
		fmt.Println("   ❓ No saved credentials for this domain.")
		email = askHumanForEmail(domain)
		if email == "" {
			fmt.Println("   ⚠️  No email provided. Leaving browser open for manual login.")
			return true
		}
		password = askHumanForPassword(domain)
		if password == "" {
			fmt.Println("   ⚠️  No password provided. Leaving browser open for manual login.")
			return true
		}

		// Save new credentials for next time
		creds[domain] = domainCredentials{Email: email, Password: password}
		saveCredentials(creds)
		fmt.Printf("   💾 Credentials saved to encrypted vault for future use.\n")
	}

	// ----- Dismiss Cookies BEFORE Filling Login Form -----
	// Many portals (like Siemens Energy) show a cookie banner ON the login page
	// that covers the login button. Must be dismissed first.
	dismissCookies(page)
	humanDelay(400, 800)

	// ----- Fill Login Form -----
	fmt.Println("   ✍️  Filling login form...")

	// Fill email/username field
	emailSelectors := []string{
		"input[type='email']",
		"input[name*='email' i]",
		"input[id*='email' i]",
		"input[name*='user' i]",
		"input[id*='user' i]",
		"input[name*='login' i]",
		"input[autocomplete='username']",
		"input[data-automation-id='email']", // Workday
	}

	emailFilled := false
	for _, sel := range emailSelectors {
		if safeFill(page, sel, email) {
			emailFilled = true
			break
		}
	}
	if !emailFilled {
		fmt.Println("   ⚠️  Could not find email/username field. Manual intervention needed.")
		return true
	}

	// Fill password field
	passwordSelectors := []string{
		"input[type='password']",
		"input[name*='password' i]",
		"input[id*='password' i]",
		"input[data-automation-id='password']", // Workday
	}

	passwordFilled := false
	for _, sel := range passwordSelectors {
		if safeFill(page, sel, password) {
			passwordFilled = true
			break
		}
	}
	if !passwordFilled {
		fmt.Println("   ⚠️  Could not find password field. Manual intervention needed.")
		return true
	}

	// ----- Click Login Button -----
	fmt.Println("   👆 Clicking login button...")

	// Dismiss cookies again right before clicking (banners can reappear)
	dismissCookies(page)
	humanDelay(200, 500)

	// Snapshot URL to detect if login succeeds
	preLoginURL := page.URL()

	loginSelectors := []string{
		// Text-based selectors FIRST (most specific, avoids hitting cookie buttons)
		"button:has-text('Login')",
		"button:has-text('Sign In')",
		"button:has-text('Sign in')",
		"button:has-text('Log In')",
		"button:has-text('Log in')",
		"button:has-text('Anmelden')",
		"button:has-text('Einloggen')",
		// Link elements (many ATS portals use <a> tags styled as buttons)
		"a:has-text('Login')",
		"a:has-text('Sign In')",
		"a:has-text('Sign in')",
		"a:has-text('Log In')",
		"a:has-text('Log in')",
		// Generic submits LAST (these can match cookie consent buttons)
		"button[type='submit']",
		"input[type='submit']",
		"input[type='submit'][value*='Sign' i]",
		"input[type='submit'][value*='Log' i]",
		"button:has-text('Continue')",
		// Platform-specific
		"button[data-automation-id='signInLink']", // Workday
		"[data-automation-id='click_filter']",     // Workday alt
	}

	loginClicked := false
	for _, sel := range loginSelectors {
		if safeClick(page, sel) {
			loginClicked = true
			break
		}
	}

	// Fallback: press Enter on the password field to submit the form
	if !loginClicked {
		fmt.Println("   ⚠️  No login button worked. Trying Enter key on password field...")
		// Try ALL password selectors, not just input[type='password']
		enterSelectors := []string{
			"input[type='password']",
			"input[name*='password' i]",
			"input[id*='password' i]",
		}
		for _, sel := range enterSelectors {
			pwField := page.Locator(sel).First()
			if count, _ := pwField.Count(); count > 0 {
				pwField.Press("Enter")
				loginClicked = true
				fmt.Println("   ⌨️  Pressed Enter on password field.")
				break
			}
		}
	}

	if !loginClicked {
		// Check if the page changed anyway (race condition: click may have worked despite error)
		humanDelay(1500, 3000)
		if page.URL() != preLoginURL {
			fmt.Println("   ✅ Page changed — login may have succeeded.")
			loginClicked = true
		} else {
			fmt.Println("   ⚠️  Could not find login button. Manual intervention needed.")
			return true
		}
	}

	// ----- Wait for Login to Complete -----
	fmt.Println("   ⏳ Waiting for login to complete...")
	page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateDomcontentloaded,
	})
	humanDelay(2000, 3000)

	// ----- CAPTCHA Check (Post-Login Click) -----
	if detectCaptcha(page) {
		if err := waitForCaptchaSolution(page, 120); err != nil {
			fmt.Printf("   ⚠️  %v\n", err)
			fmt.Println("   Leaving browser open for manual intervention.")
			return true
		}
		humanDelay(2000, 3000)
	}

	// ----- MFA Check (Post-Login) -----
	if detectMFA(page) {
		if err := waitForMFA(page, 180); err != nil {
			fmt.Printf("   ⚠️  %v\n", err)
			fmt.Println("   Leaving browser open for manual intervention.")
			return true
		}
		humanDelay(2000, 3000)
	}

	// ----- Check for Login Failure -----
	// If a password field still exists, login probably failed
	if detectAuthWall(page) {
		fmt.Println("   ❌ Login appears to have FAILED (still on login page).")
		fmt.Println("   ⚠️  Possible wrong password. Leaving browser open for manual intervention.")
		fmt.Println("   💡 TIP: Credentials will be re-prompted on next run.")

		// Remove the bad credentials so they're not reused
		delete(creds, domain)
		saveCredentials(creds)
		fmt.Println("   🗑️  Removed bad credentials from vault.")
		return true
	}

	fmt.Println("   ✅ Login successful!")

	// ----- Redirect Correction -----
	// ATS portals often redirect to a dashboard after login. Force back to the job URL.
	currentURL = page.URL()
	if !strings.Contains(currentURL, jobURL) && currentURL != jobURL {
		fmt.Printf("   🔄 Redirected to dashboard (%s). Navigating back to job...\n", currentURL)
		page.Goto(jobURL)
		page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateDomcontentloaded,
		})
		humanDelay(2000, 4000)
	}

	return true
}
