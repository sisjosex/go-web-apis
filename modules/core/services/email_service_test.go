package services

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	authConfig "josex/web/modules/auth/config"
	coreConfig "josex/web/modules/core/config"
)

// smtpOff is the service with SMTP_ENABLED=false in the given gin mode.
func smtpOff(mode string) *emailService {
	return &emailService{auth: &authConfig.AuthConfig{}, core: &coreConfig.CoreConfig{AppMode: mode}}
}

// A send that did not happen is never a silent success (AUTH-002): SMTP off is logged and counts as
// sent outside release, is ErrEmailDisabled in release; a missing template is ErrEmailTemplate always.
func TestSendEmail_UnsentIsAnError(t *testing.T) {
	tpl := filepath.Join(t.TempDir(), "mail.html")
	if err := os.WriteFile(tpl, []byte("<p>{{.Link}}</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing.html")
	data := map[string]string{"Link": "https://app.example/reset?token=x"}

	cases := []struct {
		name     string
		svc      *emailService
		template string
		want     error
	}{
		{"smtp off outside release is logged", smtpOff("debug"), tpl, nil},
		{"smtp off in release", smtpOff("release"), tpl, ErrEmailDisabled},
		{"missing template outside release", smtpOff("debug"), missing, ErrEmailTemplate},
		{"missing template in release", smtpOff("release"), missing, ErrEmailTemplate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.svc.SendEmail("someone@example.com", "subject", c.template, data)
			if c.want == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.want != nil && !errors.Is(err, c.want) {
				t.Fatalf("want %v, got %v", c.want, err)
			}
		})
	}
}

func TestSendPlainEmail_SmtpOffInRelease(t *testing.T) {
	if err := smtpOff("release").SendPlainEmail("someone@example.com", "s", "b"); !errors.Is(err, ErrEmailDisabled) {
		t.Fatalf("want ErrEmailDisabled, got %v", err)
	}
	if err := smtpOff("debug").SendPlainEmail("someone@example.com", "s", "b"); err != nil {
		t.Fatalf("want nil outside release, got %v", err)
	}
}
