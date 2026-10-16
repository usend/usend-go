package usend

import (
	"errors"
	"net"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// DefaultSMTPHost is the hosted usend SMTP submission host.
const DefaultSMTPHost = "smtp.usend.et"

// SMTPConfig holds the settings for sending through usend over SMTP. The API
// key is the password. Port 465 is implicit TLS, port 587 is STARTTLS; usend
// never offers AUTH before TLS.
type SMTPConfig struct {
	Host        string
	Port        int
	Username    string // always "usend"
	Password    string // the API key
	ImplicitTLS bool   // true for 465
}

// Addr returns host:port.
func (c SMTPConfig) Addr() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) }

// Auth returns a PLAIN auth for net/smtp (use it after TLS is on).
func (c SMTPConfig) Auth() smtp.Auth { return smtp.PlainAuth("", c.Username, c.Password, c.Host) }

// NewSMTPConfig builds SMTP settings from an API key. port is 465 or 587 (0
// means 465). The host is derived from baseURL ("https://api.x.com" gives
// "smtp.x.com"; empty gives smtp.usend.et); USEND_SMTP_HOST overrides it.
func NewSMTPConfig(apiKey, baseURL string, port int) (SMTPConfig, error) {
	if apiKey == "" {
		apiKey = os.Getenv("USEND_API_KEY")
	}
	if apiKey == "" {
		return SMTPConfig{}, errors.New("usend: missing API key for SMTP")
	}
	if port == 0 {
		port = 465
	}
	if port != 465 && port != 587 {
		return SMTPConfig{}, errors.New("usend accepts SMTP on port 465 (TLS) or 587 (STARTTLS)")
	}
	host := os.Getenv("USEND_SMTP_HOST")
	if host == "" {
		host = smtpHostFor(baseURL)
	}
	return SMTPConfig{Host: host, Port: port, Username: "usend", Password: apiKey, ImplicitTLS: port == 465}, nil
}

// SMTPConfig returns SMTP settings for this client's key and base URL. port is
// 465 or 587 (0 means 465).
func (c *Client) SMTPConfig(port int) (SMTPConfig, error) {
	cfg, err := NewSMTPConfig(c.apiKey, c.baseURL, port)
	return cfg, err
}

func smtpHostFor(baseURL string) string {
	if baseURL == "" {
		return DefaultSMTPHost
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return DefaultSMTPHost
	}
	h := u.Hostname()
	if rest, ok := strings.CutPrefix(h, "api."); ok {
		return "smtp." + rest
	}
	return DefaultSMTPHost
}
