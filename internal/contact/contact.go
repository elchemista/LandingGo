package contact

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	mailgun "github.com/mailgun/mailgun-go/v5"

	"github.com/elchemista/LandingGo/internal/config"
)

// Message represents a contact form submission.
type Message struct {
	Name  string
	Email string
	Body  string
}

// Sender defines behaviour required to deliver a contact message.
type Sender interface {
	Enabled() bool
	Send(ctx context.Context, msg Message) error
}

// Service sends contact messages using Mailgun.
type Service struct {
	cfg config.Contact
	mg  mailgun.Mailgun
}

// NewService constructs a Service using the provided configuration. When cfg is
// enabled and no explicit Mailgun client is supplied, a default client is created.
func NewService(cfg config.Contact, mg mailgun.Mailgun) *Service {
	if mg == nil && cfg.Enabled() {
		mg = mailgun.NewMailgun(cfg.Mailgun.APIKey)
	}
	return &Service{cfg: cfg, mg: mg}
}

// Enabled reports whether the service has sufficient configuration to send messages.
func (s *Service) Enabled() bool {
	if s == nil {
		return false
	}
	return s.mg != nil && s.cfg.Enabled()
}

// Send delivers a contact message via Mailgun.
func (s *Service) Send(ctx context.Context, msg Message) error {
	if s == nil {
		return errors.New("contact service is nil")
	}
	if !s.Enabled() {
		return errors.New("contact service disabled")
	}

	msg.Name = strings.TrimSpace(msg.Name)
	msg.Email = strings.TrimSpace(msg.Email)
	msg.Body = strings.TrimSpace(msg.Body)

	if msg.Name == "" || msg.Email == "" || msg.Body == "" {
		return errors.New("name, email, and message are required")
	}

	if !strings.Contains(msg.Email, "@") {
		return errors.New("sender email must contain '@'")
	}

	subject := s.cfg.Subject
	if subject == "" {
		subject = fmt.Sprintf("New contact from %s", msg.Name)
	}

	message := mailgun.NewMessage(s.cfg.Mailgun.Domain, s.cfg.From, subject, buildPlainText(msg))
	if err := message.AddRecipient(s.cfg.Recipient); err != nil {
		return fmt.Errorf("add recipient: %w", err)
	}
	message.SetReplyTo(msg.Email)
	message.AddHeader("X-Originating-Email", msg.Email)

	if _, err := s.mg.Send(ctx, message); err != nil {
		return fmt.Errorf("mailgun send: %w", err)
	}

	return nil
}

func buildPlainText(msg Message) string {
	var b strings.Builder
	b.WriteString("Name: ")
	b.WriteString(msg.Name)
	b.WriteString("\nEmail: ")
	b.WriteString(msg.Email)
	b.WriteString("\nSubmitted: ")
	b.WriteString(time.Now().UTC().Format(time.RFC3339))
	b.WriteString("\n\n")
	b.WriteString(msg.Body)
	return b.String()
}
