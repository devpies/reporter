package main

import "fmt"

// showUsage displays usage information.
func showUsage() {
	_, _ = fmt.Println("Usage: rp (reporter) [OPTIONS]")
	_, _ = fmt.Println()
	_, _ = fmt.Println("Reporter recursively reports and resolves drifts across multiple git repositories.")
	_, _ = fmt.Println()
	_, _ = fmt.Println("Options:")
	_, _ = fmt.Println("  --explain, -e     Show examples")
	_, _ = fmt.Println("  --help, -h        Show this help message")
	_, _ = fmt.Println("  --update, -u      Automatically update repositories that are behind")
	_, _ = fmt.Println("  --branch, -b      Specify the branch to check (default: main)")
	_, _ = fmt.Println("  --log, -l         Show the complete list of changes using git log")
	_, _ = fmt.Println("  --force, -f       Forcefully abort rebase and merge conflicts to update")
	_, _ = fmt.Println("  --remote, -r      Remote name (default: origin)")
}

// showExamples displays usage with example output.
func showExamples() {
	header := "  %s\n  [Remote] mvp-service is 13 commits ahead.\n  Last local commit by Lois Lane" +
		" Fri Nov 24 10:56:42 2023 +0100\n"
	message := "  abc123 fix: provide db transaction context\n%s"
	_, _ = fmt.Println()
	_, _ = fmt.Println("Examples")
	_, _ = fmt.Println()
	_, _ = fmt.Println("In a Git repository:")
	_, _ = fmt.Print("  $ rp\n\n")
	_, _ = fmt.Println("  Checking Repository For Updates git: (origin/main)")
	_, _ = fmt.Printf(header, LightRed)
	_, _ = fmt.Printf(message, Reset)
	_, _ = fmt.Println()
	_, _ = fmt.Println("In a directory containing multiple Git repositories:")
	_, _ = fmt.Print("  $ rp\n\n")
	_, _ = fmt.Println("  Checking Repositories For Updates. git: (origin/main)")
	_, _ = fmt.Println()
	_, _ = fmt.Println("  Outdated Repositories:")
	_, _ = fmt.Printf(header, LightRed)
	_, _ = fmt.Printf(message, Reset)
	_, _ = fmt.Println()
	_, _ = fmt.Print("  Up-to-Date Repositories:\n\n")
	_, _ = fmt.Printf("  %smvp-frontend is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Printf("  %smvp-backend-go is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Printf("  %smvp-backend-python is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Printf("  %smvp-shared-library is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Printf("  %smvp-tools is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Println()
	_, _ = fmt.Println("Updating a directory containing multiple Git repositories:")
	_, _ = fmt.Print("  $ rp -u\n\n")
	_, _ = fmt.Println("  Checking Repositories For Updates. git: (origin/main)")
	_, _ = fmt.Println()
	_, _ = fmt.Println("  Outdated Repositories:")
	_, _ = fmt.Printf(header, LightRed)
	_, _ = fmt.Printf(message, Reset)
	_, _ = fmt.Println("  :.")
	_, _ = fmt.Println("   Stashing local changes")
	_, _ = fmt.Println("   Pulling latest changes")
	_, _ = fmt.Println("   Applying stashed changes")
	_, _ = fmt.Printf("   %smvp-service is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Println()
	_, _ = fmt.Print("  Up-to-Date Repositories:\n\n")
	_, _ = fmt.Printf("  %smvp-frontend is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Printf("  %smvp-backend-go is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Printf("  %smvp-backend-python is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Printf("  %smvp-shared-library is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Printf("  %smvp-tools is up-to-date%s\n", LightGreen, Reset)
	_, _ = fmt.Println()
}
