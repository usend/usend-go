// Send through usend over SMTP with net/smtp (STARTTLS on 587).
// Run: USEND_API_KEY=us_... go run ./examples/smtp
package main

import (
	"log"
	"net/smtp"

	usend "usend.et/sdk/go"
)

func main() {
	client, err := usend.New("")
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := client.SMTPConfig(587) // 465 is implicit TLS: use crypto/tls.Dial and smtp.NewClient instead
	if err != nil {
		log.Fatal(err)
	}
	msg := []byte("From: Acme <hello@acme.com>\r\n" +
		"To: ann@example.com\r\n" +
		"Subject: Hello over SMTP\r\n\r\n" +
		"It works.\r\n")
	// smtp.SendMail upgrades with STARTTLS before AUTH, which is what usend requires.
	if err := smtp.SendMail(cfg.Addr(), cfg.Auth(), "hello@acme.com", []string{"ann@example.com"}, msg); err != nil {
		log.Fatal(err)
	}
	log.Println("sent")
}
