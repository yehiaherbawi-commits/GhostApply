package main

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
)

func main() {
	err := playwright.Install()
	pw, err := playwright.Run()
	if err != nil {
		panic(err)
	}
	browser, _ := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	page, _ := browser.NewPage()
	page.Goto("https://jobs.fraunhofer.de/job/Freiburg-Student-assistant-Module-Analytics-79110/1365433933/")
	
	// Try to find the button
	loc := page.Locator(`button:has-text("Accept"), button:has-text("Akzeptieren"), button:has-text("Zustimmen"), button:has-text("Alle akzeptieren")`).First()
	if count, _ := loc.Count(); count > 0 {
		loc.Click()
	}
	
	page.WaitForTimeout(3000)
	
	text, _ := page.Locator("#content").First().InnerText()
	fmt.Println("CONTENT:")
	fmt.Println(text)
	
	browser.Close()
	pw.Stop()
}
