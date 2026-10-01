package services

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"josex/web/config"
	authConfig "josex/web/modules/auth/config"
	coreConfig "josex/web/modules/core/config"
	"log"
	"net/smtp"
	"os"
	"path/filepath"
)

// EmailService handles email sending operations
type EmailService interface {
	SendEmail(to string, subject string, templatePath string, data interface{}) error
	SendPlainEmail(to string, subject string, body string) error
}

// A send that did not happen is an error, never a silent success (AUTH-002): a caller that stores a
// token before sending rolls it back on these, and the user is told.
var (
	// ErrEmailDisabled: SMTP_ENABLED=false in release mode. Outside release the email is logged instead.
	ErrEmailDisabled = errors.New("email sending is disabled (SMTP_ENABLED=false)")
	// ErrEmailTemplate: the template file is missing or does not parse — in every mode.
	ErrEmailTemplate = errors.New("email template unavailable")
)

// emailService reads the SMTP settings on every send, not once at construction: a service built at
// startup follows the live config (and a test can take SMTP down under the shared router).
type emailService struct {
	auth *authConfig.AuthConfig
	core *coreConfig.CoreConfig
}

// NewEmailService creates a new email service instance
func NewEmailService() EmailService {
	return &emailService{auth: config.ModularAppConfig.Auth, core: config.ModularAppConfig.Core}
}

// SendEmail sends an email using an HTML template
// templatePath: relative path from project root (e.g., "modules/auth/templates/verify-email.html")
// data: data to populate the template
func (s *emailService) SendEmail(to string, subject string, templatePath string, data interface{}) error {
	// The template first, in every mode: a missing one is a broken build, not a missing SMTP server.
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Printf("❌ email to %s: template %s: %v", to, templatePath, err)
		return fmt.Errorf("%w: %s: %v", ErrEmailTemplate, templatePath, err)
	}
	if !s.auth.SMTPEnabled {
		return s.skip(to, subject, data)
	}

	// Execute template with data
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		log.Printf("Error executing email template: %v", err)
		return fmt.Errorf("error executing email template: %w", err)
	}

	// Send the email
	return s.send(to, subject, body.String())
}

// SendPlainEmail sends a plain text email
func (s *emailService) SendPlainEmail(to string, subject string, body string) error {
	if !s.auth.SMTPEnabled {
		return s.skip(to, subject, body)
	}
	return s.send(to, subject, body)
}

// skip is SMTP_ENABLED=false (D1): outside release the email goes to the log — the link or code in data
// is how a developer follows the flow — and counts as sent; in release it is ErrEmailDisabled.
func (s *emailService) skip(to string, subject string, data interface{}) error {
	if s.core.AppMode == "release" {
		log.Printf("❌ email to %s not sent: SMTP_ENABLED=false", to)
		return ErrEmailDisabled
	}
	log.Printf("📧 SMTP off (dev): email to %s, subject %q, data %v", to, subject, data)
	return nil
}

// send handles the actual SMTP sending
func (s *emailService) send(to string, subject string, body string) error {
	c := s.auth
	// Build email message
	msg := []byte(
		"From: " + c.SMTPFrom + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/html; charset=UTF-8\r\n" +
			"\r\n" +
			body + "\r\n")

	// SMTP authentication
	auth := smtp.PlainAuth("", c.SMTPUser, c.SMTPPass, c.SMTPHost)

	// Send email
	addr := fmt.Sprintf("%s:%d", c.SMTPHost, c.SMTPPort)
	err := smtp.SendMail(addr, auth, c.SMTPFrom, []string{to}, msg)
	if err != nil {
		log.Printf("Error sending email to %s: %v", to, err)
		return fmt.Errorf("error sending email: %w", err)
	}

	log.Printf("Email sent successfully to %s", to)
	return nil
}

// GetTemplatePath builds a template file path that works from any directory.
// It searches for the template starting from the current directory and moving up.
func GetTemplatePath(module string, templateName string) string {
	// Try relative paths from current directory
	candidates := []string{
		filepath.Join("modules", module, "templates", templateName),
		filepath.Join("..", "modules", module, "templates", templateName),
		filepath.Join("..", "..", "modules", module, "templates", templateName),
		filepath.Join("..", "..", "..", "modules", module, "templates", templateName),
		filepath.Join("..", "..", "..", "..", "modules", module, "templates", templateName),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}

	// If not found, return the default path (will fail at runtime with clear error)
	return filepath.Join("modules", module, "templates", templateName)
}
