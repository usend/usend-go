// A webhook receiver. Run: USEND_WEBHOOK_SECRET=whsec_... go run ./examples/webhook
package main

import (
	"io"
	"log"
	"net/http"
	"os"

	usend "usend.et/sdk/go"
)

func main() {
	secret := os.Getenv("USEND_WEBHOOK_SECRET")

	http.HandleFunc("/webhooks/usend", func(w http.ResponseWriter, r *http.Request) {
		// Verify against the raw body, byte for byte.
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		event, err := usend.VerifyWebhook(body, r.Header, secret)
		if err != nil {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
		log.Printf("event %s (%s)", event.Type, event.ID)
		w.WriteHeader(http.StatusOK) // store the event, process it in the background
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
