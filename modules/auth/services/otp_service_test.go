package services

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	authModels "josex/web/modules/auth/models"
	otp "josex/web/modules/auth/services/otp"

	"github.com/google/uuid"
)

// fakeOtpRepository records what the service stores and invalidates.
type fakeOtpRepository struct {
	relayEmail  *string
	requestErr  error
	stored      []storedOtp
	invalidated []uuid.UUID
}

type storedOtp struct {
	destination, channel, code string
	relay                      bool
}

func (r *fakeOtpRepository) RequestOtp(_ context.Context, destination, channel, code string, relay bool) (*authModels.OtpRequest, error) {
	if r.requestErr != nil {
		return nil, r.requestErr
	}
	r.stored = append(r.stored, storedOtp{destination, channel, code, relay})
	rec := &authModels.OtpRequest{Id: uuid.New(), Destination: destination, OtpChannel: channel, ExpiresAt: time.Now().Add(10 * time.Minute)}
	if relay {
		rec.RelayEmail = r.relayEmail
	}
	return rec, nil
}

func (r *fakeOtpRepository) InvalidateOtp(_ context.Context, id uuid.UUID) error {
	r.invalidated = append(r.invalidated, id)
	return nil
}

func (r *fakeOtpRepository) VerifyOtp(context.Context, authModels.VerifyOtpDto) (*authModels.SessionUser, error) {
	return &authModels.SessionUser{}, nil
}

// fakeProvider records where it sent and can be told to fail.
type fakeProvider struct {
	channel string
	enabled bool
	sendErr error
	sentTo  []string
}

func (p *fakeProvider) SendOtp(_ context.Context, destination, _ string, _ string) error {
	p.sentTo = append(p.sentTo, destination)
	return p.sendErr
}
func (p *fakeProvider) GetChannelName() string { return p.channel }
func (p *fakeProvider) IsEnabled() bool        { return p.enabled }

func newService(repo *fakeOtpRepository, providers ...*fakeProvider) *OtpServiceImpl {
	registry := map[string]otp.OtpProvider{}
	for _, p := range providers {
		registry[p.channel] = p
	}
	return NewOtpService(repo, registry, nil)
}

func request(s *OtpServiceImpl, destination, channel string) (*authModels.RequestOtpResponse, error) {
	return s.RequestOtp(context.Background(), authModels.RequestOtpDto{Destination: destination, Channel: channel})
}

func TestRequestOtpRelaysPhoneChannelToAccountEmail(t *testing.T) {
	email := "rider@test.com"
	repo := &fakeOtpRepository{relayEmail: &email}
	mail := &fakeProvider{channel: "email", enabled: true}
	s := newService(repo, mail)

	for _, channel := range []string{"sms", "whatsapp"} {
		resp, err := request(s, "+59170000001", channel)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", channel, err)
		}
		if resp.Channel != channel || resp.DeliveredVia != "email" || resp.Destination != "r****r@test.com" {
			t.Errorf("%s: got channel=%q delivered_via=%q destination=%q", channel, resp.Channel, resp.DeliveredVia, resp.Destination)
		}
	}
	for i, st := range repo.stored {
		if !st.relay || st.destination != "+59170000001" {
			t.Errorf("stored[%d] = %+v, want the phone with relay", i, st)
		}
	}
	if len(mail.sentTo) != 2 || mail.sentTo[0] != email {
		t.Errorf("sent to %v, want the account email twice", mail.sentTo)
	}
}

func TestRequestOtpUsesChannelProviderWhenEnabled(t *testing.T) {
	repo := &fakeOtpRepository{}
	mail := &fakeProvider{channel: "email", enabled: true}
	sms := &fakeProvider{channel: "sms", enabled: true}
	s := newService(repo, mail, sms)

	resp, err := request(s, "+59170000001", "sms")
	if err != nil {
		t.Fatal(err)
	}
	if resp.DeliveredVia != "sms" || repo.stored[0].relay || len(mail.sentTo) != 0 || len(sms.sentTo) != 1 {
		t.Errorf("a registered SMS provider must end the relay: resp=%+v stored=%+v", resp, repo.stored)
	}
}

func TestRequestOtpChannelDisabled(t *testing.T) {
	// Email is registered but not configured: nothing can send, and nothing may be stored.
	for _, channel := range []string{"email", "sms", "whatsapp"} {
		repo := &fakeOtpRepository{}
		s := newService(repo, &fakeProvider{channel: "email"})
		if _, err := request(s, "x@test.com", channel); !errors.Is(err, ErrChannelDisabled) {
			t.Errorf("%s: got %v, want ErrChannelDisabled", channel, err)
		}
		if len(repo.stored) != 0 {
			t.Errorf("%s: stored a code no provider can send", channel)
		}
	}
}

func TestRequestOtpFailedSendInvalidatesStoredCode(t *testing.T) {
	repo := &fakeOtpRepository{}
	s := newService(repo, &fakeProvider{channel: "email", enabled: true, sendErr: errors.New("smtp down")})

	_, err := request(s, "user@test.com", "email")
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("got %v, want ErrProviderUnavailable", err)
	}
	if len(repo.stored) != 1 || len(repo.invalidated) != 1 {
		t.Errorf("stored %d, invalidated %d; want the code stored before the send and invalidated after it failed", len(repo.stored), len(repo.invalidated))
	}
}

func TestRequestOtpStoreErrorSendsNothing(t *testing.T) {
	mail := &fakeProvider{channel: "email", enabled: true}
	s := newService(&fakeOtpRepository{requestErr: errors.New("otp.phone.not-registered")}, mail)

	if _, err := request(s, "+59170000009", "sms"); err == nil {
		t.Fatal("want the SP error")
	}
	if len(mail.sentTo) != 0 {
		t.Errorf("sent to %v after the SP refused", mail.sentTo)
	}
}

func TestGenerateOtpCodeIsSixDigits(t *testing.T) {
	sixDigits := regexp.MustCompile(`^\d{6}$`)
	seen := map[string]bool{}
	for range 200 {
		code, err := generateOtpCode()
		if err != nil {
			t.Fatal(err)
		}
		if !sixDigits.MatchString(code) {
			t.Fatalf("code %q is not 6 digits", code)
		}
		seen[code] = true
	}
	if len(seen) < 190 {
		t.Errorf("only %d distinct codes in 200 draws", len(seen))
	}
}
