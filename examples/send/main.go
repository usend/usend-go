// Send one email. Run: USEND_API_KEY=us_... go run ./examples/send
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	usend "usend.et/sdk/go"
)

func main() {
	client, err := usend.New("") // reads USEND_API_KEY
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := client.Emails.Send(ctx, &usend.SendEmailParams{
		From:    "Acme <hello@acme.com>",
		To:      []string{"ann@example.com"},
		Subject: "Hello from Go",
		HTML:    "<p>It works.</p>",
		Tags:    []usend.Tag{{Name: "category", Value: "welcome"}},
	}, usend.WithIdempotencyKey("welcome-ann-1"))
	if err != nil {
		var apiErr *usend.Error
		if errors.As(err, &apiErr) {
			log.Fatalf("usend rejected the email: %s (%d): %s", apiErr.Name, apiErr.StatusCode, apiErr.Message)
		}
		log.Fatal(err)
	}
	fmt.Println("queued", res.ID)
}
