package otp

import (
	"context"
	"fmt"
	"josex/web/modules/auth/config"
	coreServices "josex/web/modules/core/services"
	coreUtils "josex/web/modules/core/utils"
	"time"
)

// EmailProvider implements OtpProvider for Email delivery via SMTP
type EmailProvider struct {
	config       *config.AuthConfig
	emailService coreServices.EmailService
	isEnabled    bool
}

// NewEmailProvider creates a new Email OTP provider
func NewEmailProvider(authConfig *config.AuthConfig) *EmailProvider {
	return &EmailProvider{
		config:       authConfig,
		emailService: coreServices.NewEmailService(),
		isEnabled:    authConfig.SMTPHost != "" && authConfig.SMTPFrom != "",
	}
}

// SendOtp sends OTP via Email using SMTP through the core email service
// Supports multilingual templates using translation keys
func (p *EmailProvider) SendOtp(ctx context.Context, destination string, otpCode string, lang string) (expiresAt time.Time, err error) {
	// Prepare template data with translated text
	templateData := map[string]string{
		"OtpCode":         otpCode,
		"Title":           coreUtils.GetTranslation(lang, "otp.email.title"),
		"Greeting":        coreUtils.GetTranslation(lang, "otp.email.greeting"),
		"Description":     coreUtils.GetTranslation(lang, "otp.email.description"),
		"CodeLabel":       coreUtils.GetTranslation(lang, "otp.email.code-label"),
		"ExpirationText":  coreUtils.GetTranslation(lang, "otp.email.expiration"),
		"SecurityWarning": coreUtils.GetTranslation(lang, "otp.email.security-warning"),
		"IgnoreText":      coreUtils.GetTranslation(lang, "otp.email.ignore-text"),
		"SignOff":         coreUtils.GetTranslation(lang, "otp.email.sign-off"),
		"AutomatedNote":   coreUtils.GetTranslation(lang, "otp.email.automated-note"),
	}

	// Subject based on language
	subject := coreUtils.GetTranslation(lang, "otp.email.subject")

	// Get template path
	templatePath := coreServices.GetTemplatePath("auth", "otp-code.html")

	// Send email using core email service with template
	if err := p.emailService.SendEmail(destination, subject, templatePath, templateData); err != nil {
		fmt.Printf("❌ Error sending OTP email to %s: %v\n", destination, err)
		return time.Time{}, fmt.Errorf("failed to send OTP email: %w", err)
	}

	fmt.Printf("📧 OTP email sent successfully to %s (lang=%s)\n", destination, lang)
	expiresAt = time.Now().Add(10 * time.Minute)
	return expiresAt, nil
}

// GetChannelName returns the channel identifier
func (p *EmailProvider) GetChannelName() string {
	return "email"
}

// IsEnabled returns whether the provider is configured and enabled
func (p *EmailProvider) IsEnabled() bool {
	return p.isEnabled
}
