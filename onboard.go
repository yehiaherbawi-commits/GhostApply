package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// RunOnboarding walks the user through an interactive setup wizard
func RunOnboarding() {
	fmt.Println("🚀 Welcome to JobAgent Onboarding!")
	fmt.Println("Let's set up your agent profile and base CV.")
	fmt.Println("---")

	reader := bufio.NewReader(os.Stdin)
	profile := make(map[string]string)

	questions := []struct {
		key      string
		question string
	}{
		{"full_name", "What is your full name?"},
		{"email", "What is your email address?"},
		{"phone_number", "What is your phone number?"},
		{"location", "Where are you currently located (City, Country)?"},
		{"target_roles", "What are your target roles (e.g., Software Engineer, AI PM)?"},
		{"salary_expectations", "What are your salary expectations (e.g., $120k, 80,000 EUR)?"},
		{"notice_period", "What is your notice period / availability?"},
		{"visa_sponsorship", "Do you require visa sponsorship? (Yes/No)"},
		{"superpowers", "What are your 2-3 biggest superpowers/skills?"},
	}

	for _, q := range questions {
		fmt.Printf("👉 %s\n> ", q.question)
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(ans)
		if ans != "" {
			profile[q.key] = ans
		}
	}

	// Save profile
	profileData, _ := json.MarshalIndent(profile, "", "  ")
	err := os.WriteFile("agent_profile.json", profileData, 0644)
	if err != nil {
		fmt.Printf("❌ Could not save agent_profile.json: %v\n", err)
	} else {
		fmt.Println("✅ Saved agent_profile.json")
	}

	// Generate baseline CV
	cvTemplate := fmt.Sprintf(`Name: %s
Email: %s
Phone: %s
Location: %s

Professional Summary:
A highly motivated professional targeting %s roles. Key strengths include %s.

Experience:
- Please add your experience here.
- Focus on accomplishments and metrics.

Education:
- Please add your education here.
`, profile["full_name"], profile["email"], profile["phone_number"], profile["location"], profile["target_roles"], profile["superpowers"])

	if _, err := os.Stat("my_cv.txt"); os.IsNotExist(err) {
		err = os.WriteFile("my_cv.txt", []byte(cvTemplate), 0644)
		if err != nil {
			fmt.Printf("❌ Could not save my_cv.txt: %v\n", err)
		} else {
			fmt.Println("✅ Generated baseline my_cv.txt (Please fill in your experience later!)")
		}
	} else {
		fmt.Println("ℹ️  my_cv.txt already exists. Skipping baseline generation.")
	}

	fmt.Println("---")
	fmt.Println("🎉 Onboarding complete! You are ready to run GhostApply.")
}
