package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// The text part says what the HTML says (APP-009): a link keeps its address, a block ends its line,
// entities are read, and nothing of the head or the markup is left.
func TestTextFromHTML(t *testing.T) {
	body := `<html><head><style>p{color:red}</style></head><body><h2>Hola,</h2>` +
		`<p>Hac&eacute; clic:</p><a class="b" href="https://app.example/reset?token=1">Restablecer</a>` +
		`<p>Saludos, el equipo de Taypi</p></body></html>`
	want := "Hola,\n\nHacé clic:\nRestablecer: https://app.example/reset?token=1\nSaludos, el equipo de Taypi"
	if got := textFromHTML(body); got != want {
		t.Fatalf("textFromHTML:\n got %q\nwant %q", got, want)
	}
}

// An HTML email is multipart/alternative with the text first; its subject is encoded word by word.
func TestMessage_Alternative(t *testing.T) {
	msg := string(message("from@x", "to@x", "Atención", "texto", "<p>html</p>"))
	for _, part := range []string{
		"Subject: =?utf-8?q?Atenci=C3=B3n?=",
		"Content-Type: multipart/alternative",
		"Content-Type: text/plain; charset=UTF-8\r\n\r\ntexto",
		"Content-Type: text/html; charset=UTF-8\r\n\r\n<p>html</p>",
	} {
		if !strings.Contains(msg, part) {
			t.Fatalf("message lacks %q:\n%s", part, msg)
		}
	}
}
