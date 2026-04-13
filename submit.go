package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// AutoFillApplication opens a visible browser and injects your data into the ATS form
func AutoFillApplication(pw *playwright.Playwright, jobURL string, finalCV *CVContent, pdfPath string, answers *AppAnswers) error {
	fmt.Println("\n🤖 [Auto-Submitter] Launching browser...")

	// Launch in non-headless mode so you can watch the AI work!
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

	fmt.Printf("🌐 [Auto-Submitter] Navigating to %s\n", jobURL)
	if _, err := page.Goto(jobURL); err != nil {
		return fmt.Errorf("could not navigate: %v", err)
	}

	// Try to find and click an "Apply" button if it exists.
	// We use a 5-second timeout. If it's not found, we assume the form is already visible.
	fmt.Println("👆 [Auto-Submitter] Looking for an 'Apply' button...")
	applyBtn := page.Locator("button:has-text('Apply'), a:has-text('Apply'), button:has-text('apply'), a:has-text('apply')").First()
	errBtn := applyBtn.Click(playwright.LocatorClickOptions{
		Timeout: playwright.Float(5000),
	})

	if errBtn == nil {
		fmt.Println("👆 [Auto-Submitter] Clicked 'Apply' button. Waiting for form...")
		time.Sleep(3 * time.Second) // wait for new page or modal form
	} else {
		// Fallback for input type buttons
		applyInput := page.Locator("input[type='button'][value*='Apply' i], input[type='submit'][value*='Apply' i]").First()
		errInput := applyInput.Click(playwright.LocatorClickOptions{
			Timeout: playwright.Float(2000),
		})
		if errInput == nil {
			fmt.Println("👆 [Auto-Submitter] Clicked 'Apply' input. Waiting for form...")
			time.Sleep(3 * time.Second)
		} else {
			// No apply button found, just wait a moment for the dynamic form to finish loading
			time.Sleep(3 * time.Second)
		}
	}

	fmt.Println("✍️  [Auto-Submitter] Injecting data into the DOM...")

	// 1. Split name into First and Last
	names := strings.SplitN(finalCV.Name, " ", 2)
	firstName := names[0]
	lastName := ""
	if len(names) > 1 {
		lastName = names[1]
	}

	// 2. Best-Effort Data Injection (Targets standard ATS field names)
	// Fill First Name
	page.Locator("input[name*='first' i], input[id*='first' i]").First().Fill(firstName)
	// Fill Last Name
	page.Locator("input[name*='last' i], input[id*='last' i]").First().Fill(lastName)
	// Fill Email
	if finalCV.Email != "" {
		page.Locator("input[type='email'], input[name*='email' i], input[id*='email' i]").First().Fill(finalCV.Email)
	}
	// Fill Phone Number
	if finalCV.Phone != "" {
		page.Locator("input[type='tel'], input[name*='phone' i], input[id*='phone' i], input[name*='tel' i]").First().Fill(finalCV.Phone)
	}
	// Fill LinkedIn
	if finalCV.LinkedIn != "" {
		linkedinLoc := page.Locator("input[name*='linkedin' i], input[id*='linkedin' i], input[name*='urls[LinkedIn]' i]")
		if count, _ := linkedinLoc.Count(); count > 0 {
			linkedinLoc.First().Fill(finalCV.LinkedIn)
		}
	}

	// 3. Upload the Tailored PDF
	// Look for the standard file upload input
	fileInput := page.Locator("input[type='file']").First()
	if count, _ := fileInput.Count(); count > 0 {
		fmt.Printf("📄 [Auto-Submitter] Uploading %s...\n", pdfPath)
		fileInput.SetInputFiles(pdfPath)
	}

	// 4. Inject the AI-Drafted Answers into Textareas
	textAreas := page.Locator("textarea")
	count, _ := textAreas.Count()

	if count > 0 && answers != nil {
		// Fill the first text area with the "Why Us" answer
		textAreas.Nth(0).Fill(answers.WhyCompany)
		fmt.Println("💬 [Auto-Submitter] Injected 'Why Us' answer.")

		if count > 1 {
			// Fill the second text area with the "Tech Challenge" answer
			textAreas.Nth(1).Fill(answers.TechChallenge)
			fmt.Println("💬 [Auto-Submitter] Injected 'Tech Challenge' answer.")
		}
	}

	// 5. Intelligent Fallback for Remaining Fields
	fmt.Println("\n🧠 [Knowledge Base] Checking remaining unknown fields...")
	
	formHTML, _ := page.Locator("form").First().InnerHTML()
	if formHTML == "" {
		formHTML, _ = page.Locator("main").First().InnerHTML()
		if formHTML == "" {
			formHTML, _ = page.Locator("body").InnerHTML()
		}
	}
	
	profileBytes, errProfile := os.ReadFile("agent_profile.json")
	if errProfile == nil {
		fmt.Println("   🤖 [AI Filler] Cross-referencing against agent_profile.json...")
		actionPlan, aiErr := AIFinishApplication(formHTML, string(profileBytes))
		if aiErr == nil && actionPlan != nil {
			for _, action := range actionPlan.Actions {
				if action.Type == "unknown_required" {
					fmt.Printf("   ⚠️  Unknown Required Field: [%s] - Skipping rather than crashing.\n", action.Label)
					continue
				}

				loc := page.Locator(action.Selector).First()
				if count, _ := loc.Count(); count > 0 {
					switch action.Type {
					case "fill":
						loc.Fill(action.Value)
						fmt.Printf("   ✅ Filled field [%s]\n", action.Selector)
					case "select":
						loc.SelectOption(playwright.SelectOptionValues{
							Labels: playwright.StringSlice(action.Value),
						})
						fmt.Printf("   ✅ Selected option [%s] in [%s]\n", action.Value, action.Selector)
					case "click":
						loc.Click()
						time.Sleep(1 * time.Second) // wait for dropdown animations
						fmt.Printf("   ✅ Clicked element [%s]\n", action.Selector)
					}
				}
			}
		} else {
			fmt.Printf("   ❌ AI Filler failed: %v\n", aiErr)
		}
	} else {
		fmt.Println("   ⚠️  No agent_profile.json found, skipping intelligent fallback.")
	}

	fmt.Println("\n✨ [Auto-Submitter] Form filled! The browser will stay open for you to review and click Submit.")
	fmt.Println("   (Close the browser window manually when you are done).")

	return nil
}
