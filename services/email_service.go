package services

import (
	"bytes"
	"html/template"
	"sync"

	"josex/web/config"

	"gopkg.in/gomail.v2"
)

var (
	dialer *gomail.Dialer
	once   sync.Once
)

// InitDialer inicializa el dialer solo una vez
func InitDialer() {
	once.Do(func() {
		smtpHost := config.AppConfig.SmtpHost
		smtpPort := config.AppConfig.SmtpPort
		smtpUser := config.AppConfig.SmtpUser
		smtpPass := config.AppConfig.SmtpPass

		dialer = gomail.NewDialer(smtpHost, smtpPort, smtpUser, smtpPass)
	})
}

// SendEmail envía un correo usando el dialer global
func SendEmail(to, subject, body string) error {
	InitDialer() // Asegurar que el dialer esté inicializado

	m := gomail.NewMessage()
	m.SetHeader("From", config.AppConfig.SmtpFrom)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	return dialer.DialAndSend(m)
}

// Datos para la plantilla
type VerificationTokenEmailData struct {
	VerificationURL string
}

type PasswordResetEmailData struct {
	PasswordResetURL string
}

// LoadTemplate carga y procesa una plantilla HTML
func LoadTemplate(templatePath string, data interface{}) (string, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", err
	}

	var tplBuffer bytes.Buffer
	if err := tmpl.Execute(&tplBuffer, data); err != nil {
		return "", err
	}

	return tplBuffer.String(), nil
}

func SendEmailVerificationToken(to, subject string, data VerificationTokenEmailData) error {
	body, err := LoadTemplate("templates/email/verify-email.html", data)
	if err != nil {
		return err
	}

	return SendEmail(to, subject, body)
}

func SendPasswordResetToken(to, subject string, data PasswordResetEmailData) error {
	body, err := LoadTemplate("templates/email/password-reset.html", data)
	if err != nil {
		return err
	}

	return SendEmail(to, subject, body)
}
