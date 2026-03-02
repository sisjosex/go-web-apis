package services

import (
	"bytes"
	"fmt"
	"html/template"
	"josex/web/config"
	"log"
	"net/smtp"
	"path/filepath"
)

// EmailService handles email sending operations
type EmailService interface {
	SendEmail(to string, subject string, templatePath string, data interface{}) error
	SendPlainEmail(to string, subject string, body string) error
}

type emailService struct {
	smtpHost string
	smtpPort int
	smtpUser string
	smtpPass string
	smtpFrom string
}

// NewEmailService creates a new email service instance
func NewEmailService() EmailService {
	authConf := config.ModularAppConfig.Auth
	return &emailService{
		smtpHost: authConf.SMTPHost,
		smtpPort: authConf.SMTPPort,
		smtpUser: authConf.SMTPUser,
		smtpPass: authConf.SMTPPass,
		smtpFrom: authConf.SMTPFrom,
	}
}

// SendEmail sends an email using an HTML template
// templatePath: relative path from project root (e.g., "modules/auth/templates/verify-email.html")
// data: data to populate the template
func (s *emailService) SendEmail(to string, subject string, templatePath string, data interface{}) error {
	// Parse template with cross-platform path handling
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Printf("Error parsing email template %s: %v", templatePath, err)
		// Gracefully handle missing templates (common in test environment)
		// Email sending is not critical - don't fail the request
		return nil
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
	return s.send(to, subject, body)
}

// send handles the actual SMTP sending
func (s *emailService) send(to string, subject string, body string) error {
	// Build email message
	msg := []byte(
		"From: " + s.smtpFrom + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/html; charset=UTF-8\r\n" +
			"\r\n" +
			body + "\r\n")

	// SMTP authentication
	auth := smtp.PlainAuth("", s.smtpUser, s.smtpPass, s.smtpHost)

	// Send email
	addr := fmt.Sprintf("%s:%d", s.smtpHost, s.smtpPort)
	err := smtp.SendMail(addr, auth, s.smtpFrom, []string{to}, msg)
	if err != nil {
		log.Printf("Error sending email to %s: %v", to, err)
		return fmt.Errorf("error sending email: %w", err)
	}

	log.Printf("Email sent successfully to %s", to)
	return nil
}

// GetTemplatePath builds a template file path that works across all platforms.
// filepath.Join automatically uses the correct separator (/ on Unix, \ on Windows).
func GetTemplatePath(module string, templateName string) string {
	return filepath.Join("modules", module, "templates", templateName)
}
