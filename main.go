package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
)

func main() {
	token := os.Getenv("BITRISE_TOKEN")
	if token == "" {
		log.Fatal("BITRISE_TOKEN environment variable is not set")
	}

	client := bitriseapi.New(token)

	user, err := client.Me(context.Background())
	if err != nil {
		log.Fatalf("me: %v", err)
	}

	fmt.Printf("Username:  %s\n", user.Username)
	fmt.Printf("Email:     %s\n", user.Email)
	fmt.Printf("Avatar:    %s\n", user.AvatarURL)
}
