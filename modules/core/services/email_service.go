package services

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"html/template"
	"josex/web/config"
	authConfig "josex/web/modules/auth/config"
	coreConfig "josex/web/modules/core/config"
	"log"
	"mime"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

	// Send the email, with the same words as plain text for clients that read that part (APP-009).
	return s.send(to, subject, textFromHTML(body.String()), body.String())
}

// SendPlainEmail sends a plain text email
func (s *emailService) SendPlainEmail(to string, subject string, body string) error {
	if !s.auth.SMTPEnabled {
		return s.skip(to, subject, body)
	}
	return s.send(to, subject, body, "")
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

// mimeBoundary separates the parts of a multipart/alternative message; no body contains it.
const mimeBoundary = "taypi-alt-7f3c9e1b"

// message is the RFC 5322 message: text alone, or text and HTML as alternatives of one another. The
// subject is RFC 2047-encoded, so its accents survive any relay.
func message(from, to, subject, text, htmlBody string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\nTo: " + to + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\nMIME-Version: 1.0\r\n")
	if htmlBody == "" {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n" + text + "\r\n")
		return []byte(b.String())
	}
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + mimeBoundary + "\"\r\n\r\n")
	b.WriteString("--" + mimeBoundary + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + text + "\r\n")
	b.WriteString("--" + mimeBoundary + "\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + htmlBody + "\r\n")
	b.WriteString("--" + mimeBoundary + "--\r\n")
	return []byte(b.String())
}

var (
	htmlDropped   = regexp.MustCompile(`(?is)<(head|style|script)[^>]*>.*?</(head|style|script)>`)
	htmlLink      = regexp.MustCompile(`(?is)<a\s[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	htmlBreak     = regexp.MustCompile(`(?i)<br\s*/?>|</?(p|div|h[1-6]|li|tr|table)(\s[^>]*)?>`)
	htmlTag       = regexp.MustCompile(`<[^>]+>`)
	blankLines    = regexp.MustCompile(`\n(\s*\n)+`)
	spacesInLines = regexp.MustCompile(`[ \t]+`)
)

// textFromHTML is a rendered template as plain text: a link becomes "text: url", a block ends its line.
func textFromHTML(body string) string {
	text := htmlDropped.ReplaceAllString(body, "")
	text = htmlLink.ReplaceAllString(text, "$2: $1")
	text = htmlBreak.ReplaceAllString(text, "\n")
	text = html.UnescapeString(htmlTag.ReplaceAllString(text, ""))
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(spacesInLines.ReplaceAllString(line, " "))
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// send handles the actual SMTP sending: the text alone when htmlBody is empty.
func (s *emailService) send(to, subject, text, htmlBody string) error {
	c := s.auth
	msg := message(c.SMTPFrom, to, subject, text, htmlBody)

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
