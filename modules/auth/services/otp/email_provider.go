package otp

import (
	"context"
	"josex/web/modules/auth/config"
	coreServices "josex/web/modules/core/services"
	coreUtils "josex/web/modules/core/utils"
)

// EmailProvider implements OtpProvider for Email delivery via SMTP
type EmailProvider struct {
	emailService coreServices.EmailService
	isEnabled    bool
}

// NewEmailProvider creates a new Email OTP provider.
// Enabled only with SMTP_ENABLED: the core email service skips sending silently otherwise, and an OTP
// that is never sent must answer 503, not 200.
func NewEmailProvider(authConfig *config.AuthConfig) *EmailProvider {
	return &EmailProvider{
		emailService: coreServices.NewEmailService(),
		isEnabled:    authConfig.SMTPEnabled && authConfig.SMTPHost != "" && authConfig.SMTPFrom != "",
	}
}

// SendOtp sends OTP via Email using SMTP through the core email service
// Supports multilingual templates using translation keys
func (p *EmailProvider) SendOtp(ctx context.Context, destination string, otpCode string, lang string) error {
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

	subject := coreUtils.GetTranslation(lang, "otp.email.subject")
	templatePath := coreServices.GetTemplatePath("auth", "otp-code.html")

	return p.emailService.SendEmail(destination, subject, templatePath, templateData)
}

// GetChannelName returns the channel identifier
func (p *EmailProvider) GetChannelName() string {
	return "email"
}

// IsEnabled returns whether the provider is configured and enabled
func (p *EmailProvider) IsEnabled() bool {
	return p.isEnabled
}
