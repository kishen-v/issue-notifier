package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/go-github/v47/github"
	"github.com/slack-go/slack"
	"golang.org/x/oauth2"
)

const (
	checkInterval = 30 * time.Minute // Check every 30 minutes
)

func main() {
	// Read environment variables
	githubToken := os.Getenv("PAT")
	if githubToken == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required but not set.")
	}

	slackToken := os.Getenv("SLACK_TOKEN")
	if slackToken == "" {
		log.Fatal("SLACK_TOKEN environment variable is required but not set.")
	}

	slackChannelID := os.Getenv("CHANNEL_ID")
	if slackChannelID == "" {
		log.Fatal("SLACK_CHANNEL_ID environment variable is required but not set.")
	}

	// Initialize GitHub client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(nil, ts)
	client := github.NewClient(tc)

	// Initialize Slack client
	slackClient := slack.New(slackToken)

	// Run forever, checking every tick (every 30 minutes)
	log.Println("Checking for new 'Good First Issue' issues...")
	checkForNewIssues(client, slackClient, slackChannelID)
}

// checkForNewIssues fetches and posts issues to Slack
func checkForNewIssues(githubClient *github.Client, slackClient *slack.Client, slackChannelID string) {
	ctx := context.Background()
	// Calculate the time range (last 30 minutes)
	now := time.Now()
	since := now.Add(-checkInterval)

	// List orgs: Kubernetes and Kubernetes SIGs
	orgs := []string{"kubernetes", "kubernetes-sigs", "etcd-io"}

	var issues []github.Issue
	for _, org := range orgs {
		// Initialize pagination
		page := 1
		for {
			// List all repositories for the org with pagination
			repos, resp, err := githubClient.Repositories.ListByOrg(ctx, org, &github.RepositoryListByOrgOptions{
				ListOptions: github.ListOptions{Page: page, PerPage: 100}, // Adjust per page if necessary
			})
			if err != nil {
				log.Fatal(err)
			}

			// Print number of repos fetched
			fmt.Printf("Fetched %d repositories for org: %s (Page %d)\n", len(repos), org, page)

			// Check each repo for issues labeled "good first issue"
			for _, repo := range repos {
				opts := &github.IssueListByRepoOptions{
					Labels: []string{"help wanted"},
					Since:  since,
					State:  "open",
				}

				// Fetch issues from the repo
				repoIssues, _, err := githubClient.Issues.ListByRepo(ctx, org, *repo.Name, opts)
				if err != nil {
					log.Println("Error fetching issues from repo:", *repo.Name, err)
					continue
				}

				// Filter issues created in the last 30 minutes
				for _, issue := range repoIssues {
					if issue.CreatedAt != nil && issue.CreatedAt.After(since) {
						issues = append(issues, *issue)
					}
				}
			}

			// Check if there are more pages
			if resp.NextPage == 0 {
				break // No more pages
			}
			page = resp.NextPage // Move to the next page
		}
	}

	// Publish the identified issues to Slack
	fmt.Printf("Found %d issues \n", len(issues))
	if len(issues) > 0 {
		var availableIssues []string
		for _, issue := range issues {
			availableIssues = append(availableIssues, *issue.HTMLURL)
		}

		// Generate a message with all the issue links
		var messageLink string
		for _, issueLink := range availableIssues {
			messageLink = messageLink + fmt.Sprintf("%s \n", issueLink)
		}

		// Send message to Slack
		err := postToSlack(slackClient, slackChannelID, fmt.Sprintf(":alert-3319: Identified good first issues! \n%s", messageLink))
		if err != nil {
			log.Printf("Failed to send message to Slack: %v", err)
		} else {
			log.Printf("Sent issues to Slack")
		}
	} else {
		log.Printf("No new 'Good First Issues' found in the last %s minutes", checkInterval)
	}
}

// postToSlack sends the message to a Slack channel using the slack-go/slack package
func postToSlack(client *slack.Client, slackChannelID, message string) error {
	_, _, err := client.PostMessage(slackChannelID,
		slack.MsgOptionText(message, false),
	)
	if err != nil {
		return fmt.Errorf("failed to send message to Slack: %w", err)
	}

	return nil
}
