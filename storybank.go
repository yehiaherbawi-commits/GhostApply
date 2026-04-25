package main

import (
	"fmt"
	"os"
	"strings"
)

func AppendStories(stories []STARStory, company string) {
	if len(stories) == 0 {
		return
	}

	filename := "story-bank.md"
	var sb strings.Builder

	// Add header if file doesn't exist
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		sb.WriteString("# Interview Story Bank\n\n")
		sb.WriteString("This file contains automatically extracted STAR+R stories from deep evaluations, categorized by the requirements they satisfy.\n\n")
	}

	sb.WriteString(fmt.Sprintf("\n## Added from Deep Evaluation: %s\n\n", company))

	for _, s := range stories {
		sb.WriteString(fmt.Sprintf("### %s\n", s.Requirement))
		sb.WriteString(fmt.Sprintf("**Situation:** %s\n\n", s.Situation))
		sb.WriteString(fmt.Sprintf("**Task:** %s\n\n", s.Task))
		sb.WriteString(fmt.Sprintf("**Action:** %s\n\n", s.Action))
		sb.WriteString(fmt.Sprintf("**Result:** %s\n\n", s.Result))
		sb.WriteString(fmt.Sprintf("**Reflection:** %s\n\n", s.Reflection))
		sb.WriteString("---\n\n")
	}

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("   ⚠️  [Story Bank] Could not append to story-bank.md: %v\n", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(sb.String()); err != nil {
		fmt.Printf("   ⚠️  [Story Bank] Error writing to story-bank.md: %v\n", err)
	} else {
		fmt.Printf("   📚 [Story Bank] Appended %d stories to story-bank.md\n", len(stories))
	}
}
