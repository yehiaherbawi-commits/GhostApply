package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// ============================================================================
// QA MEMORY BANK — Self-Learning Answer Cache
// ============================================================================

const qaMemoryFile = "qa_memory.json"

// loadQAMemory reads the memory bank from disk. Creates it if missing.
func loadQAMemory() map[string]string {
	memory := make(map[string]string)

	data, err := os.ReadFile(qaMemoryFile)
	if err != nil {
		saveQAMemory(memory)
		fmt.Println("   📝 Created new qa_memory.json (empty)")
		return memory
	}

	if err := json.Unmarshal(data, &memory); err != nil {
		fmt.Printf("   ⚠️  qa_memory.json is corrupted, starting fresh: %v\n", err)
		memory = make(map[string]string)
		saveQAMemory(memory)
	}

	return memory
}

// saveQAMemory persists the memory bank to disk immediately
func saveQAMemory(memory map[string]string) {
	data, _ := json.MarshalIndent(memory, "", "  ")
	os.WriteFile(qaMemoryFile, data, 0644)
}

// findInMemory checks if a question (label text) has a matching answer in memory.
// It does fuzzy matching by checking if the memory key is contained in the label or vice-versa.
func findInMemory(memory map[string]string, label string) (string, bool) {
	normalizedLabel := strings.ToLower(strings.TrimSpace(label))

	// Exact match first
	if val, ok := memory[normalizedLabel]; ok {
		return val, true
	}

	// Fuzzy: check if any memory key is a substring of the label or vice-versa
	for key, val := range memory {
		normalizedKey := strings.ToLower(key)
		if strings.Contains(normalizedLabel, normalizedKey) || strings.Contains(normalizedKey, normalizedLabel) {
			return val, true
		}
	}

	return "", false
}

// askHuman pauses execution and prompts the user in the terminal for an answer
func askHuman(question string) string {
	fmt.Printf("\n   ⚠️  New Question Found: \"%s\"\n", question)
	fmt.Print("   ➡️  Please type your answer (or press Enter to skip): ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	return answer
}

// ============================================================================
// ATS PLATFORM DETECTION
// ============================================================================

// detectATS examines the page URL and HTML to identify the ATS platform
func detectATS(page playwright.Page) string {
	pageURL := page.URL()

	switch {
	case strings.Contains(pageURL, "personio"):
		return "Personio"
	case strings.Contains(pageURL, "greenhouse"):
		return "Greenhouse"
	case strings.Contains(pageURL, "lever.co") || strings.Contains(pageURL, "jobs.lever"):
		return "Lever"
	case strings.Contains(pageURL, "workday") || strings.Contains(pageURL, "myworkdayjobs"):
		return "Workday"
	case strings.Contains(pageURL, "smartrecruiters"):
		return "SmartRecruiters"
	case strings.Contains(pageURL, "breezy"):
		return "Breezy"
	case strings.Contains(pageURL, "ashbyhq"):
		return "Ashby"
	case strings.Contains(pageURL, "icims"):
		return "iCIMS"
	case strings.Contains(pageURL, "successfactors") || strings.Contains(pageURL, "siemens-energy.com"):
		return "SAP SuccessFactors"
	}

	// Fallback: check for meta tags or known DOM signatures
	bodyText, _ := page.Locator("body").InnerText()
	if strings.Contains(bodyText, "Powered by Greenhouse") {
		return "Greenhouse"
	}
	if strings.Contains(bodyText, "Powered by Lever") {
		return "Lever"
	}
	if strings.Contains(bodyText, "SuccessFactors") {
		return "SAP SuccessFactors"
	}

	return "Unknown"
}

// ============================================================================
// COOKIE BANNER AUTO-DISMISS
// ============================================================================

// dismissCookies attempts to click any cookie consent "Accept" button on the page
func dismissCookies(page playwright.Page) {
	cookieSelectors := []string{
		// Common cookie consent buttons
		"button:has-text('Accept All')",
		"button:has-text('Accept all')",
		"button:has-text('Accept Cookies')",
		"button:has-text('Accept cookies')",
		"button:has-text('Alle akzeptieren')",
		"button:has-text('Alle Cookies akzeptieren')",
		"button:has-text('Allow All')",
		"button:has-text('Allow all')",
		"button:has-text('I Accept')",
		"button:has-text('I agree')",
		"button:has-text('Agree')",
		"button:has-text('OK')",
		"button:has-text('Got it')",
		"a:has-text('Accept All')",
		"a:has-text('Accept all')",
		// Common cookie consent IDs and classes
		"button#onetrust-accept-btn-handler",
		"button.accept-cookies",
		"button[data-action='accept']",
		"button[aria-label*='accept' i]",
		"button[aria-label*='cookie' i]",
		"div.cookie-banner button",
		"#cookie-consent-accept",
		"#CybotCookiebotDialogBodyLevelButtonLevelOptinAllowAll",
	}

	for _, sel := range cookieSelectors {
		loc := page.Locator(sel).First()
		if count, _ := loc.Count(); count > 0 {
			err := loc.Click(playwright.LocatorClickOptions{
				Timeout: playwright.Float(2000),
			})
			if err == nil {
				fmt.Println("🍪 [Cookies] Accepted cookie consent banner.")
				time.Sleep(1 * time.Second)
				return
			}
		}
	}
}

// ============================================================================
// SAFE PLAYWRIGHT WRAPPERS — Never Crash
// ============================================================================

// safeFill wraps Locator.Fill() with error recovery.
// It also includes a JS-based fallback using document.getElementById() to
// handle portals (like Siemens Energy) whose numeric IDs fail CSS escaping.
func safeFill(page playwright.Page, selector string, value string) bool {
	loc := page.Locator(selector).First()
	count, _ := loc.Count()
	if count > 0 {
		err := loc.Fill(value)
		if err == nil {
			return true
		}
	}

	// JS fallback: try document.getElementById for numeric IDs (Siemens, etc.)
	// Selector pattern: '#12345' or 'select#12345' or 'textarea#12345'
	if strings.HasPrefix(selector, "#") || strings.Contains(selector, "#") {
		// Extract the raw ID (after the #)
		idPart := selector
		if idx := strings.Index(idPart, "#"); idx >= 0 {
			idPart = idPart[idx+1:]
		}
		// Unescape CSS: '\32 9476' -> '29476'
		idPart = strings.ReplaceAll(idPart, `\32 `, "2")
		idPart = strings.ReplaceAll(idPart, `\3`, "")
		idPart = strings.TrimSpace(idPart)

		_, err := page.Evaluate(fmt.Sprintf(`() => {
			const el = document.getElementById(%q);
			if (!el) return false;
			const nativeInputValueSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
			const nativeTextareaSetter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value');
			if (nativeTextareaSetter) {
				nativeTextareaSetter.set.call(el, %q);
			} else {
				nativeInputValueSetter.call(el, %q);
			}
			el.dispatchEvent(new Event('input', { bubbles: true }));
			el.dispatchEvent(new Event('change', { bubbles: true }));
			return true;
		}`, idPart, value, value), nil)
		if err == nil {
			return true
		}
	}

	if count == 0 {
		return false
	}
	fmt.Printf("   ⚠️  [Safe] Failed to fill '%s'\n", selector)
	return false
}

// safeSelect wraps Locator.SelectOption() with error recovery.
// It first checks if the element is a real native <select> before trying SelectOption.
func safeSelect(page playwright.Page, selector string, value string) bool {
	loc := page.Locator(selector).First()
	count, _ := loc.Count()
	if count == 0 {
		return false
	}

	// Check if this is actually a native <select> element
	tagName, _ := loc.Evaluate("el => el.tagName", nil)
	if tagStr, ok := tagName.(string); !ok || strings.ToLower(tagStr) != "select" {
		// Not a native select — don't waste time, let combobox handle it
		return false
	}

	// Try selecting by label first (with short timeout)
	_, err := loc.SelectOption(playwright.SelectOptionValues{
		Labels: playwright.StringSlice(value),
	})
	if err == nil {
		return true
	}

	// Retry with exact value attribute
	_, err = loc.SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(value),
	})
	if err == nil {
		return true
	}

	// Try all available options for a partial/case-insensitive match
	options, evalErr := loc.Evaluate("el => Array.from(el.options).map(o => o.text.trim())", nil)
	if evalErr == nil {
		if optSlice, ok := options.([]interface{}); ok {
			valueLower := strings.ToLower(value)
			for _, opt := range optSlice {
				if optStr, ok := opt.(string); ok {
					optLower := strings.ToLower(optStr)
					if optLower == valueLower || strings.Contains(optLower, valueLower) || strings.Contains(valueLower, optLower) {
						_, matchErr := loc.SelectOption(playwright.SelectOptionValues{
							Labels: playwright.StringSlice(optStr),
						})
						if matchErr == nil {
							fmt.Printf("   ✅ [Select Fuzzy] Matched '%s' → '%s'\n", value, optStr)
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// safeClick wraps Locator.Click() with error recovery
func safeClick(page playwright.Page, selector string) bool {
	loc := page.Locator(selector).First()
	count, _ := loc.Count()
	if count == 0 {
		return false
	}
	err := loc.Click(playwright.LocatorClickOptions{
		Timeout: playwright.Float(3000),
	})
	if err != nil {
		fmt.Printf("   ⚠️  [Safe] Failed to click '%s': %v\n", selector, err)
		return false
	}
	return true
}

// ============================================================================
// PAGINATION — "Next" Button Detection
// ============================================================================

// findAndClickNext scans for forward-progression buttons and clicks one.
// It NEVER clicks Submit/Send/Finish/Apply — those are terminal actions.
// Returns true if a "Next" button was found and clicked.
func findAndClickNext(page playwright.Page) bool {
	// Words that indicate FORWARD PROGRESSION (safe to click)
	nextPatterns := []string{
		"Next", "Continue", "Save and Continue", "Save & Continue",
		"Weiter", "Fortfahren", // German
		"Proceed", "Next Step", "Next Page",
	}

	// Words that indicate TERMINAL ACTIONS (NEVER click these)
	excludePatterns := []string{
		"submit", "send", "finish", "apply", "confirm", "complete",
		"absenden", "einreichen", "bewerben", // German
	}

	// Scan buttons and links
	allButtons := page.Locator("button, a, input[type='submit']")
	count, _ := allButtons.Count()

	for i := 0; i < count; i++ {
		btn := allButtons.Nth(i)

		// Get the button text
		text, err := btn.InnerText()
		if err != nil {
			// For input[type='submit'], try the value attribute
			text, _ = btn.GetAttribute("value")
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		textLower := strings.ToLower(text)

		// Check if this button is a TERMINAL action — skip it
		isExcluded := false
		for _, exclude := range excludePatterns {
			if strings.Contains(textLower, exclude) {
				isExcluded = true
				break
			}
		}
		if isExcluded {
			continue
		}

		// Check if this button matches a FORWARD-PROGRESSION pattern
		isNext := false
		for _, pattern := range nextPatterns {
			if strings.Contains(textLower, strings.ToLower(pattern)) {
				isNext = true
				break
			}
		}

		// Also check for ">" arrow-style buttons
		if text == ">" || text == "›" || text == "→" {
			isNext = true
		}

		if isNext {
			// Verify the button is visible and enabled
			if visible, _ := btn.IsVisible(); !visible {
				continue
			}
			if disabled, _ := btn.IsDisabled(); disabled {
				continue
			}

			fmt.Printf("   ➡️  Found 'Next' button: \"%s\" — clicking...\n", text)
			err := btn.Click(playwright.LocatorClickOptions{
				Timeout: playwright.Float(5000),
			})
			if err != nil {
				fmt.Printf("   ⚠️  Failed to click next button: %v\n", err)
				return false
			}
			return true
		}
	}

	return false
}

// ============================================================================
// VISUAL-FIRST FIELD SCANNER — Works on native AND custom ATS widgets
// ============================================================================

type formField struct {
	Label     string   `json:"label"`     // Human-readable question text
	FieldType string   `json:"fieldType"` // "text", "dropdown", "radio", "textarea", "checkbox"
	TagName   string   `json:"tagName"`   // kept for backward compat with Phase 3 UI
	Required  bool     `json:"required"`  // Whether the field is required
	Options   []string `json:"options"`   // Available options (for select/radio)
	Selector  string   `json:"selector"`  // Optional CSS selector hint (may be empty)
}

// smartInject fills a field using a visual-first approach.
// It does NOT rely on CSS selectors or native HTML tags. Instead it:
// 1. Finds the field by visible label text on screen
// 2. Determines the interactive element type
// 3. Uses the appropriate fill strategy (type, click-to-reveal, toggle)
// smartInject uses Playwright's Proximity XPath and a Click-to-Reveal fallback
// to handle complex enterprise ATS platforms (like SuccessFactors) that heavily nest inputs and iframes.
func smartInject(page playwright.Page, field formField, answer string) bool {
	variants := getAnswerVariants(answer)
	cleanLabel := strings.TrimSuffix(strings.TrimSpace(field.Label), "*")
	cleanLabel = strings.TrimSpace(cleanLabel)

	// 1. IFRAME AWARENESS: SuccessFactors uses child frames.
	// Search main page and all child frames.
	var frames []playwright.Frame
	frames = append(frames, page.MainFrame())
	frames = append(frames, page.MainFrame().ChildFrames()...)

	for _, frame := range frames {
		// Use Playwright Proximity XPath to find the interactive element directly following the label
		// Note: we look for input, textarea, select OR role=combobox / role=listbox / role=radio
		xpath := fmt.Sprintf("xpath=//*[contains(text(), '%s')]/following::*[self::input or self::textarea or @role='combobox' or @role='radio' or @role='listbox' or self::select][1]", cleanLabel)
		target := frame.Locator(xpath).First()

		count, _ := target.Count()
		if count == 0 {
			continue // Not found in this frame, try next
		}

		// Determine the type of the element
		tagNameVal, err := target.Evaluate("el => el.tagName.toLowerCase()", nil)
		if err != nil {
			continue
		}
		tagName := tagNameVal.(string)
		roleVal, _ := target.GetAttribute("role")

		// Strategy: Fill Text
		if tagName == "input" || tagName == "textarea" {
			// If it's a radio or checkbox input, handle it differently if needed,
			// but for now try to fill or handle based on type
			inputType, _ := target.GetAttribute("type")
			if inputType == "radio" || inputType == "checkbox" {
				// Broad visual fill radio fallback or direct click
				target.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)})
				fmt.Printf("   ✅ [Proximity] Clicked input '%s' for answer '%s'\n", cleanLabel, answer)
				return true
			}

			err := target.Fill(answer)
			if err == nil {
				fmt.Printf("   ✅ [Proximity] Filled text '%s' → '%s'\n", cleanLabel, answer)
				return true
			}
		}

		// Strategy: Native Select
		if tagName == "select" {
			_, err := target.SelectOption(playwright.SelectOptionValues{Labels: playwright.StringSlice(answer)})
			if err == nil {
				fmt.Printf("   ✅ [Proximity Select] '%s' → '%s'\n", cleanLabel, answer)
				return true
			}
			// Try variants
			for _, v := range variants {
				_, err := target.SelectOption(playwright.SelectOptionValues{Labels: playwright.StringSlice(v)})
				if err == nil {
					fmt.Printf("   ✅ [Proximity Select Variant] '%s' → '%s'\n", cleanLabel, v)
					return true
				}
			}
		}

		// Strategy: Click-to-Reveal Fallback (Combobox / Listbox)
		if roleVal == "combobox" || roleVal == "listbox" {
			// a. Click to force dropdown open
			err := target.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)})
			if err != nil {
				continue
			}

			// b. Wait for React/Angular to render options
			page.WaitForTimeout(500)

			// c. Search page for exact text of answer
			for _, v := range variants {
				// Search inside the frame
				optionLoc := frame.GetByText(v, playwright.FrameGetByTextOptions{Exact: playwright.Bool(true)}).First()
				if optCount, _ := optionLoc.Count(); optCount > 0 {
					if vis, _ := optionLoc.IsVisible(); vis {
						// d. Click that exact text option
						if clickErr := optionLoc.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)}); clickErr == nil {
							fmt.Printf("   ✅ [Click-to-Reveal] '%s' → '%s'\n", cleanLabel, v)
							return true
						}
					}
				}
			}

			// e. Press Escape to close menu if not found
			page.Keyboard().Press("Escape")
			page.WaitForTimeout(300)
		}

		// Strategy: Role Radio
		if roleVal == "radio" {
			target.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)})
			fmt.Printf("   ✅ [Proximity] Clicked radio '%s'\n", cleanLabel)
			return true
		}
	}

	fmt.Printf("   ⚠️  [Proximity] Could not inject answer for '%s'\n", cleanLabel)
	return false
}

// ============================================================================
// VISUAL FILL STRATEGIES — Work on native AND custom ATS widgets
// ============================================================================

// visualFillText fills a text input or textarea found by its visible label.
func visualFillText(page playwright.Page, field formField, answer string) bool {
	cleanLabel := strings.TrimSuffix(strings.TrimSpace(field.Label), "*")
	cleanLabel = strings.TrimSpace(cleanLabel)

	// Strategy 1: Playwright's GetByLabel (handles for=, aria-labelledby, parent <label>)
	byLabel := page.GetByLabel(cleanLabel)
	if count, _ := byLabel.Count(); count > 0 {
		first := byLabel.First()
		if visible, _ := first.IsVisible(); visible {
			err := first.Fill(answer)
			if err == nil {
				return true
			}
			// Fill failed — try Click + Type
			first.Click()
			page.Keyboard().Press("Control+a")
			page.Keyboard().Type(answer)
			return true
		}
	}

	// Strategy 2: XPath — find label text, then next input/textarea
	labelLoc := page.Locator(fmt.Sprintf(
		"xpath=//label[contains(text(),'%s')] | //span[contains(text(),'%s')] | //div[contains(text(),'%s')]",
		cleanLabel, cleanLabel, cleanLabel,
	)).First()
	if count, _ := labelLoc.Count(); count > 0 {
		nextInput := labelLoc.Locator("xpath=following::input[not(@type='hidden') and not(@type='submit') and not(@type='radio') and not(@type='checkbox')][1] | following::textarea[1]").First()
		if ic, _ := nextInput.Count(); ic > 0 {
			if visible, _ := nextInput.IsVisible(); visible {
				err := nextInput.Fill(answer)
				if err == nil {
					return true
				}
			}
		}
	}

	// Strategy 3: CSS selector fallback (if scanner provided one)
	if field.Selector != "" {
		return safeFill(page, field.Selector, answer)
	}

	fmt.Printf("   ⚠️  [Visual] Could not fill text field '%s'\n", field.Label)
	return false
}

// visualFillDropdown handles native <select>, custom combobox, click-to-reveal dropdowns.
func visualFillDropdown(page playwright.Page, field formField, answerVariants []string) bool {
	cleanLabel := strings.TrimSuffix(strings.TrimSpace(field.Label), "*")
	cleanLabel = strings.TrimSpace(cleanLabel)

	for _, answer := range answerVariants {
		// Strategy 1: Native <select> via GetByLabel
		byLabel := page.GetByLabel(cleanLabel)
		if count, _ := byLabel.Count(); count > 0 {
			first := byLabel.First()
			if tag, err := first.Evaluate("el => el.tagName", nil); err == nil {
				if tagStr, ok := tag.(string); ok && strings.EqualFold(tagStr, "SELECT") {
					_, err := first.SelectOption(playwright.SelectOptionValues{Labels: playwright.StringSlice(answer)})
					if err == nil {
						fmt.Printf("   ✅ [Native Select] '%s' → '%s'\n", field.Label, answer)
						return true
					}
					if fuzzySelectOption(first, answer) {
						return true
					}
				}
			}
		}

		// Strategy 2: XPath to find dropdown trigger near the label, then click-to-reveal
		labelLoc := page.Locator(fmt.Sprintf(
			"xpath=//label[contains(text(),'%s')] | //span[contains(text(),'%s')] | //div[contains(text(),'%s')]",
			cleanLabel, cleanLabel, cleanLabel,
		)).First()
		if count, _ := labelLoc.Count(); count == 0 {
			continue
		}

		dropdownXPaths := []string{
			"following::select[1]",
			"following::*[@role='combobox'][1]",
			"following::*[@role='listbox'][1]",
			"following::*[@aria-haspopup='listbox'][1]",
			"following::*[contains(@class,'dropdown')][1]",
			"following::*[contains(@class,'combobox')][1]",
			"following::button[1]",
			"following::input[1]",
		}

		for _, xp := range dropdownXPaths {
			target := labelLoc.Locator("xpath=" + xp).First()
			if tc, _ := target.Count(); tc == 0 {
				continue
			}
			if visible, _ := target.IsVisible(); !visible {
				continue
			}

			// Native select found via XPath
			if tag, err := target.Evaluate("el => el.tagName", nil); err == nil {
				if tagStr, ok := tag.(string); ok && strings.EqualFold(tagStr, "SELECT") {
					_, err := target.SelectOption(playwright.SelectOptionValues{Labels: playwright.StringSlice(answer)})
					if err == nil {
						fmt.Printf("   ✅ [Select XPath] '%s' → '%s'\n", field.Label, answer)
						return true
					}
					if fuzzySelectOption(target, answer) {
						return true
					}
					continue
				}
			}

			// Click-to-Reveal: open the dropdown
			err := target.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)})
			if err != nil {
				continue
			}
			time.Sleep(600 * time.Millisecond)

			// Type to filter
			focused := page.Locator("input:focus").First()
			if fc, _ := focused.Count(); fc > 0 {
				focused.Fill(answer)
			} else {
				page.Keyboard().Type(answer)
			}
			time.Sleep(600 * time.Millisecond)

			// Scan for matching options in revealed listbox
			optionSels := []string{"[role='option']", "[role='listbox'] li", "li[class*='option' i]", "div[class*='option' i]"}
			for _, optSel := range optionSels {
				opts := page.Locator(optSel)
				oc, _ := opts.Count()
				for j := 0; j < oc; j++ {
					opt := opts.Nth(j)
					if vis, _ := opt.IsVisible(); !vis {
						continue
					}
					optText, _ := opt.InnerText()
					if strings.Contains(strings.ToLower(strings.TrimSpace(optText)), strings.ToLower(answer)) {
						if clickErr := opt.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)}); clickErr == nil {
							fmt.Printf("   ✅ [Dropdown] '%s' → '%s'\n", field.Label, answer)
							return true
						}
					}
				}
			}

			// Fallback: GetByText on entire page
			textOpt := page.GetByText(answer, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}).First()
			if tc, _ := textOpt.Count(); tc > 0 {
				if vis, _ := textOpt.IsVisible(); vis {
					if clickErr := textOpt.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)}); clickErr == nil {
						fmt.Printf("   ✅ [Dropdown Text] '%s' → '%s'\n", field.Label, answer)
						return true
					}
				}
			}

			page.Keyboard().Press("Escape")
			time.Sleep(300 * time.Millisecond)
		}
	}

	fmt.Printf("   ⚠️  [Dropdown] Could not select option for '%s'\n", field.Label)
	return false
}

// visualFillRadio handles native radio buttons, custom toggles, and Yes/No widgets.
func visualFillRadio(page playwright.Page, field formField, answerVariants []string) bool {
	cleanLabel := strings.TrimSuffix(strings.TrimSpace(field.Label), "*")
	cleanLabel = strings.TrimSpace(cleanLabel)

	for _, answer := range answerVariants {
		// Strategy 1: Scoped — find question label, get container, click answer text within it
		labelLoc := page.Locator(fmt.Sprintf(
			"xpath=//label[contains(text(),'%s')] | //span[contains(text(),'%s')] | //div[contains(text(),'%s')] | //legend[contains(text(),'%s')]",
			cleanLabel, cleanLabel, cleanLabel, cleanLabel,
		)).First()
		if count, _ := labelLoc.Count(); count > 0 {
			container := labelLoc.Locator("xpath=ancestor::div[1] | ancestor::fieldset[1] | ancestor::li[1]").First()
			if cc, _ := container.Count(); cc > 0 {
				// Text match within container
				answerOpt := container.GetByText(answer, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)}).First()
				if ac, _ := answerOpt.Count(); ac > 0 {
					if vis, _ := answerOpt.IsVisible(); vis {
						if err := answerOpt.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)}); err == nil {
							fmt.Printf("   ✅ [Radio] '%s' → '%s'\n", field.Label, answer)
							return true
						}
					}
				}

				// Native radio inputs
				radios := container.Locator("input[type='radio']")
				rCount, _ := radios.Count()
				for i := 0; i < rCount; i++ {
					radio := radios.Nth(i)
					radioID, _ := radio.GetAttribute("id")
					var radioLabel string
					if radioID != "" {
						lbl := page.Locator(fmt.Sprintf("label[for='%s']", radioID))
						if lc, _ := lbl.Count(); lc > 0 {
							radioLabel, _ = lbl.InnerText()
						}
					}
					if radioLabel == "" {
						parent := radio.Locator("xpath=ancestor::label[1]")
						if lc, _ := parent.Count(); lc > 0 {
							radioLabel, _ = parent.InnerText()
						}
					}
					if strings.EqualFold(strings.TrimSpace(radioLabel), answer) {
						radio.Check()
						fmt.Printf("   ✅ [Radio Native] '%s' → '%s'\n", field.Label, answer)
						return true
					}
				}

				// Custom role="radio" widgets
				customRadios := container.Locator("[role='radio']")
				crCount, _ := customRadios.Count()
				for i := 0; i < crCount; i++ {
					cr := customRadios.Nth(i)
					crText, _ := cr.InnerText()
					if strings.EqualFold(strings.TrimSpace(crText), answer) {
						cr.Click()
						fmt.Printf("   ✅ [Radio Custom] '%s' → '%s'\n", field.Label, answer)
						return true
					}
				}
			}
		}

		// Strategy 2: Broad — page-wide search for the answer text
		answerLoc := page.GetByText(answer, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}).First()
		if ac, _ := answerLoc.Count(); ac > 0 {
			if vis, _ := answerLoc.IsVisible(); vis {
				if err := answerLoc.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)}); err == nil {
					fmt.Printf("   ✅ [Radio Broad] '%s' → '%s'\n", field.Label, answer)
					return true
				}
			}
		}
	}

	fmt.Printf("   ⚠️  [Radio] Could not select option for '%s'\n", field.Label)
	return false
}

// fuzzySelectOption tries to select from a native <select> using case-insensitive matching
func fuzzySelectOption(loc playwright.Locator, answer string) bool {
	opts, err := loc.Evaluate("el => Array.from(el.options).map(o => o.text.trim())", nil)
	if err != nil {
		return false
	}
	optSlice, ok := opts.([]interface{})
	if !ok {
		return false
	}
	answerLower := strings.ToLower(answer)
	for _, opt := range optSlice {
		optStr, ok := opt.(string)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(optStr), answerLower) || strings.Contains(answerLower, strings.ToLower(optStr)) {
			_, err := loc.SelectOption(playwright.SelectOptionValues{Labels: playwright.StringSlice(optStr)})
			if err == nil {
				fmt.Printf("   ✅ [Select Fuzzy] '%s'\n", optStr)
				return true
			}
		}
	}
	return false
}

// getAnswerVariants returns the original answer plus DE↔EN translations.
func getAnswerVariants(answer string) []string {
	variants := []string{answer}
	translations := map[string]string{
		"deutschland": "Germany", "germany": "Deutschland",
		"jordanien": "Jordan", "jordan": "Jordanien",
		"österreich": "Austria", "austria": "Österreich",
		"schweiz": "Switzerland", "switzerland": "Schweiz",
		"frankreich": "France", "france": "Frankreich",
		"niederlande": "Netherlands", "netherlands": "Niederlande",
		"vereinigtes königreich": "United Kingdom", "united kingdom": "Vereinigtes Königreich",
		"vereinigte staaten": "United States", "united states": "Vereinigte Staaten",
		"spanien": "Spain", "spain": "Spanien",
		"italien": "Italy", "italy": "Italien",
		"polen": "Poland", "poland": "Polen",
		"türkei": "Turkey", "turkey": "Türkei",
		"schweden": "Sweden", "sweden": "Schweden",
		"norwegen": "Norway", "norway": "Norwegen",
		"dänemark": "Denmark", "denmark": "Dänemark",
		"belgien": "Belgium", "belgium": "Belgien",
		"indien": "India", "india": "Indien",
		"kanada": "Canada", "canada": "Kanada",
		"australien": "Australia", "australia": "Australien",
		"ja": "Yes", "yes": "Ja",
		"nein": "No", "no": "Nein",
	}
	if translated, ok := translations[strings.ToLower(answer)]; ok {
		variants = append(variants, translated)
	}
	return variants
}

// ============================================================================
// MAIN ENTRY POINT — AutoFillApplication
// ============================================================================

// AutoFillApplication opens a visible browser and fills the ATS form using
// the CV data, AI-drafted answers, agent_profile.json, and the self-learning
// QA memory bank. It handles auth walls autonomously. It NEVER clicks Submit.
func AutoFillApplication(pw *playwright.Playwright, jobURL string, finalCV *CVContent, pdfPath string, answers *AppAnswers) error {
	fmt.Println("\n🤖 [Auto-Submitter] Launching browser...")

	// ----- Load Knowledge Sources -----
	qaMemory := loadQAMemory()
	fmt.Printf("   📚 Loaded QA Memory Bank (%d remembered answers)\n", len(qaMemory))

	profileData := make(map[string]string)
	if profileBytes, err := os.ReadFile("agent_profile.json"); err == nil {
		json.Unmarshal(profileBytes, &profileData)
		fmt.Printf("   📋 Loaded Agent Profile (%d fields)\n", len(profileData))
	}

	// ----- Launch Visible Browser -----
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
	})
	if err != nil {
		return fmt.Errorf("could not launch browser: %v", err)
	}

	page, err := browser.NewPage()
	if err != nil {
		return fmt.Errorf("could not create page: %v", err)
	}

	// ----- Navigate -----
	fmt.Printf("🌐 [Auto-Submitter] Navigating to %s\n", jobURL)
	if _, err := page.Goto(jobURL); err != nil {
		return fmt.Errorf("could not navigate: %v", err)
	}

	// ----- Cookie Banner Auto-Accept -----
	dismissCookies(page)

	// ----- ATS Detection -----
	atsName := detectATS(page)
	fmt.Printf("🏢 [ATS Detected] %s\n", atsName)

	// ================================================================
	// AUTH WALL DETECTION & BYPASS
	// ================================================================
	authHandled := handleAuthWall(page, jobURL)
	if authHandled {
		// Re-detect ATS after potential redirect from login
		atsName = detectATS(page)

		// If we're still on a login page after attempting, bail gracefully
		if detectAuthWall(page) {
			fmt.Println("\n⚠️  Still on login page. Please log in manually in the browser window.")
			fmt.Println("   The browser will stay open for manual intervention.")
			return nil
		}
	}

	// ----- Portal Sign-In Gate (email-only gates like Personio) -----
	loginEmail := page.Locator("input[type='email'], input[name*='email' i], input[id*='email' i]").First()
	if count, _ := loginEmail.Count(); count > 0 {
		formInputs := page.Locator("input[name*='first' i], input[name*='last' i], input[name*='phone' i]")
		formCount, _ := formInputs.Count()
		if formCount == 0 {
			fmt.Println("🔑 [Portal] Sign-in gate detected. Filling login email...")
			loginEmail.Fill("yehia.herbawi@gmail.com")
			continueBtn := page.Locator("button[type='submit'], button:has-text('Continue'), button:has-text('Sign'), button:has-text('Log'), button:has-text('Next'), a:has-text('Continue')").First()
			if err := continueBtn.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)}); err == nil {
				fmt.Println("🔑 [Portal] Clicked continue. Waiting for form to load...")
				time.Sleep(4 * time.Second)
			}
		}
	}

	// ----- Click "Apply" Button (if present) -----
	fmt.Println("👆 [Auto-Submitter] Looking for an 'Apply' button...")
	applyBtn := page.Locator("button:has-text('Apply'), a:has-text('Apply'), button:has-text('apply'), a:has-text('apply')").First()
	errBtn := applyBtn.Click(playwright.LocatorClickOptions{
		Timeout: playwright.Float(5000),
	})

	if errBtn == nil {
		fmt.Println("👆 [Auto-Submitter] Clicked 'Apply' button. Waiting for page...")
		time.Sleep(3 * time.Second)
	} else {
		applyInput := page.Locator("input[type='button'][value*='Apply' i], input[type='submit'][value*='Apply' i]").First()
		errInput := applyInput.Click(playwright.LocatorClickOptions{
			Timeout: playwright.Float(2000),
		})
		if errInput == nil {
			fmt.Println("👆 [Auto-Submitter] Clicked 'Apply' input. Waiting for page...")
			time.Sleep(3 * time.Second)
		} else {
			time.Sleep(3 * time.Second)
		}
	}

	// ----- Post-Apply Cookie Check -----
	dismissCookies(page)

	// ----- Post-Apply Auth Wall Check -----
	// Many ATS portals (like Siemens) redirect to a login page AFTER clicking Apply
	postApplyAuth := handleAuthWall(page, jobURL)
	if postApplyAuth {
		// Give the page time to process login before re-checking
		time.Sleep(2 * time.Second)

		if detectAuthWall(page) {
			fmt.Println("\n⚠️  Still on login page after Apply. Please log in manually.")
			fmt.Println("   The browser will stay open for manual intervention.")
			fmt.Println("   💡 TIP: Log in manually, then close the browser when done.")
			// Wait for the user to manually log in — poll every 5 seconds
			fmt.Println("   ⏳ Waiting for you to log in manually...")
			for i := 0; i < 60; i++ { // Wait up to 5 minutes
				time.Sleep(5 * time.Second)
				if !detectAuthWall(page) {
					fmt.Println("   ✅ Login detected! Resuming automation...")
					break
				}
				if i == 59 {
					fmt.Println("   ⏰ Timed out waiting for manual login. Stopping.")
					return nil
				}
			}
		}
		// After successful login, we may need to click Apply again
		fmt.Println("👆 [Auto-Submitter] Re-checking for 'Apply' button after login...")
		dismissCookies(page)
		reApply := page.Locator("button:has-text('Apply'), a:has-text('Apply')").First()
		if err := reApply.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(5000)}); err == nil {
			fmt.Println("👆 [Auto-Submitter] Clicked 'Apply' again. Waiting for form...")
			time.Sleep(3 * time.Second)
			dismissCookies(page)
		}
	}

	// ================================================================
	// MULTI-STEP PAGINATION LOOP
	// Handles enterprise ATS platforms (Workday, SuccessFactors, Taleo)
	// that split applications across multiple pages.
	// ================================================================

	maxSteps := 10
	newQuestionsLearned := 0
	pdfUploaded := false

	// Skip labels that are clearly already handled deterministically
	skipLabels := map[string]bool{
		"first name": true, "last name": true, "name": true,
		"email": true, "e-mail": true, "phone": true,
		"telephone": true, "mobile": true, "resume": true,
		"cv": true, "password": true,
	}

	names := strings.SplitN(finalCV.Name, " ", 2)
	firstName := names[0]
	lastName := ""
	if len(names) > 1 {
		lastName = names[1]
	}

	for step := 1; step <= maxSteps; step++ {
		fmt.Printf("\n══════════════════════════════════════════════════════\n")
		fmt.Printf("📄 [Step %d/%d] Processing current page...\n", step, maxSteps)
		fmt.Println("══════════════════════════════════════════════════════")

		// ============================================================
		// PHASE 1 — Deterministic Baseline (Known CV Fields)
		// ============================================================
		fmt.Println("\n✍️  [Phase 1] Injecting known CV data...")

		safeFill(page, "input[name*='first' i], input[id*='first' i]", firstName)
		safeFill(page, "input[name*='last' i], input[id*='last' i]", lastName)

		if finalCV.Email != "" {
			safeFill(page, "input[type='email'], input[name*='email' i], input[id*='email' i]", finalCV.Email)
		}
		if finalCV.Phone != "" {
			safeFill(page, "input[type='tel'], input[name*='phone' i], input[id*='phone' i], input[name*='tel' i]", finalCV.Phone)
		}
		if finalCV.LinkedIn != "" {
			safeFill(page, "input[name*='linkedin' i], input[id*='linkedin' i], input[name*='urls[LinkedIn]' i]", finalCV.LinkedIn)
		}

		// Upload the tailored PDF (only once across all steps)
		if !pdfUploaded {
			fileInput := page.Locator("input[type='file']").First()
			if count, _ := fileInput.Count(); count > 0 {
				fmt.Printf("   📄 Uploading %s...\n", pdfPath)
				fileInput.SetInputFiles(pdfPath)
				pdfUploaded = true
			}
		}

		// Inject AI-drafted answers into textareas (only if present on this step)
		if answers != nil {
			textAreas := page.Locator("textarea")
			taCount, _ := textAreas.Count()
			if taCount > 0 {
				textAreas.Nth(0).Fill(answers.WhyCompany)
				fmt.Println("   💬 Injected 'Why Us' answer.")
			}
			if taCount > 1 {
				textAreas.Nth(1).Fill(answers.TechChallenge)
				fmt.Println("   💬 Injected 'Tech Challenge' answer.")
			}
		}

		// ============================================================
		// PHASE 2 — AI-Powered Profile Fallback (agent_profile.json)
		// ============================================================
		fmt.Println("\n🧠 [Phase 2] AI scanning form for profile-matching fields...")

		formHTML, _ := page.Locator("form").First().InnerHTML()
		if formHTML == "" {
			formHTML, _ = page.Locator("main").First().InnerHTML()
			if formHTML == "" {
				formHTML, _ = page.Locator("body").InnerHTML()
			}
		}

		if len(profileData) > 0 {
			profileJSON, _ := json.Marshal(profileData)
			actionPlan, aiErr := AIFinishApplication(formHTML, string(profileJSON))
			if aiErr == nil && actionPlan != nil {
				for _, action := range actionPlan.Actions {
					switch action.Type {
					case "unknown_required":
						fmt.Printf("   ⚠️  Unknown Required Field: [%s] — Skipping rather than crashing.\n", action.Label)
					case "fill":
						if safeFill(page, action.Selector, action.Value) {
							fmt.Printf("   ✅ AI filled [%s]\n", action.Label)
						}
					case "select":
						if safeSelect(page, action.Selector, action.Value) {
							fmt.Printf("   ✅ AI selected [%s] → %s\n", action.Label, action.Value)
						}
					case "click":
						if safeClick(page, action.Selector) {
							time.Sleep(1 * time.Second)
							fmt.Printf("   ✅ AI clicked [%s]\n", action.Label)
						}
					}
				}
			} else if aiErr != nil {
				fmt.Printf("   ⚠️  AI Profile Filler error: %v\n", aiErr)
			}
		}

		// ============================================================
		// PHASE 3 — Self-Learning QA Memory Bank
		// ============================================================
		fmt.Println("\n🔍 [Phase 3] Scanning ALL remaining form fields...")
		time.Sleep(1 * time.Second)

		fields := scanFormFields(page)
		fmt.Printf("   📋 Found %d scannable fields on this step\n", len(fields))

		for _, field := range fields {
			normalizedLabel := strings.ToLower(strings.TrimSpace(field.Label))

			if skipLabels[normalizedLabel] {
				continue
			}

			// ----- MEMORY CHECK -----
			if answer, found := findInMemory(qaMemory, field.Label); found {
				fmt.Printf("   🧠 [Memory Hit] \"%s\" → \"%s\"\n", field.Label, answer)
				smartInject(page, field, answer)
				continue
			}

			// ----- PROFILE CHECK -----
			if answer, found := findInMemory(profileData, field.Label); found {
				fmt.Printf("   📋 [Profile Hit] \"%s\" → \"%s\"\n", field.Label, answer)
				smartInject(page, field, answer)
				qaMemory[normalizedLabel] = answer
				saveQAMemory(qaMemory)
				continue
			}

			// ----- UNKNOWN FIELD -----
			if !field.Required {
				fmt.Printf("   ⏭️  Optional field \"%s\" has no answer — skipping.\n", field.Label)
				continue
			}

			// Required field with no known answer — ask the human
			fmt.Printf("\n   ┌─────────────────────────────────────────────────\n")
			fmt.Printf("   │ 🆕 REQUIRED FIELD: \"%s\" [%s]\n", field.Label, field.FieldType)
			if len(field.Options) > 0 {
				fmt.Printf("   │ 📋 Available options: %v\n", field.Options)
			}
			answer := askHuman(field.Label)

			if answer == "" {
				fmt.Printf("   │ ⏭️  Skipped (no answer provided)\n")
				fmt.Printf("   └─────────────────────────────────────────────────\n")
				continue
			}

			smartInject(page, field, answer)

			// LEARN: Save this Q&A pair forever
			qaMemory[normalizedLabel] = answer
			saveQAMemory(qaMemory)
			newQuestionsLearned++
			fmt.Printf("   │ 💾 Saved to memory! Will auto-fill next time.\n")
			fmt.Printf("   └─────────────────────────────────────────────────\n")
		}

		// ============================================================
		// NEXT BUTTON DETECTION — Navigate to next step or break
		// ============================================================
		fmt.Println("\n🔎 [Pagination] Looking for forward-progression button...")

		// Snapshot current state to detect if page actually changes
		preClickURL := page.URL()
		preClickTitle, _ := page.Title()

		nextClicked := findAndClickNext(page)
		if !nextClicked {
			fmt.Printf("   🏁 No 'Next' button found on step %d. This appears to be the final page.\n", step)
			break
		}

		fmt.Println("   ⏳ Transitioning to next step...")
		page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateDomcontentloaded,
		})
		time.Sleep(2 * time.Second)
		dismissCookies(page)

		// --- Stuck Page Detection ---
		postClickURL := page.URL()
		postClickTitle, _ := page.Title()
		if postClickURL == preClickURL && postClickTitle == preClickTitle {
			// Check if the page content actually changed by looking at visible headings
			fmt.Println("   ⚠️  Page did not change after clicking 'Next'. Possibly stuck (validation errors?).")
			fmt.Println("   🏁 Breaking pagination loop. Please check the browser for errors.")
			break
		}
	}

	// ================================================================
	// DONE — Human-in-the-Loop (Never Submit)
	// ================================================================
	fmt.Println("\n══════════════════════════════════════════════════════")
	fmt.Printf("✨ [Auto-Submitter] Reached the final step! Form filling complete.\n")
	fmt.Printf("   📚 QA Memory Bank: %d total remembered answers", len(qaMemory))
	if newQuestionsLearned > 0 {
		fmt.Printf(" (+%d new!)", newQuestionsLearned)
	}
	fmt.Println()
	fmt.Printf("   🏢 ATS Platform: %s\n", atsName)
	fmt.Println("   🛑 The SUBMIT button was NOT clicked.")
	fmt.Println("   👀 Review the form in the browser and submit manually.")
	fmt.Println("   (Close the browser window when you are done)")
	fmt.Println("══════════════════════════════════════════════════════")

	return nil
}
