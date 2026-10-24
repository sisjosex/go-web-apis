package services

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// ErrGoogleSignIn is a code Google refused, or an ID token that does not name this app and a verified address.
var ErrGoogleSignIn = errors.New("google sign-in refused")

// GoogleIdentity is the account a Google sign-in proved (APP-016).
type GoogleIdentity struct {
	Subject   string
	Email     string
	FirstName string
	LastName  string
}

// GoogleSignIn turns the code from the web popup into the account behind it.
type GoogleSignIn interface {
	Exchange(ctx context.Context, code string) (*GoogleIdentity, error)
}

type googleClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	jwt.RegisteredClaims
}

var googleIssuers = []string{"accounts.google.com", "https://accounts.google.com"}

type googleSignIn struct {
	oauth *oauth2.Config
}

// NewGoogleSignIn exchanges at Google's token endpoint, or at tokenURL when set (tests point it at a fake).
func NewGoogleSignIn(clientID, clientSecret, tokenURL string) GoogleSignIn {
	// Google's endpoint, spelled out: golang.org/x/oauth2/google would pull the GCP metadata client in.
	endpoint := oauth2.Endpoint{
		AuthURL:   "https://accounts.google.com/o/oauth2/auth",
		TokenURL:  "https://oauth2.googleapis.com/token",
		AuthStyle: oauth2.AuthStyleInParams,
	}
	if tokenURL != "" {
		endpoint.TokenURL = tokenURL
	}
	return &googleSignIn{oauth: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     endpoint,
		// The GIS popup code flow has no redirect page: Google expects the literal "postmessage".
		RedirectURL: "postmessage",
	}}
}

func (g *googleSignIn) Exchange(ctx context.Context, code string) (*GoogleIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	token, err := g.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGoogleSignIn, err)
	}
	raw, _ := token.Extra("id_token").(string)
	if raw == "" {
		return nil, fmt.Errorf("%w: no id_token", ErrGoogleSignIn)
	}

	// The token came straight from Google's token endpoint over TLS, authenticated with the client
	// secret, so its signature need not be checked again (OpenID Connect Core 3.1.3.7); its
	// audience, issuer and expiry still are.
	claims := &googleClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(raw, claims); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGoogleSignIn, err)
	}
	if !slices.Contains(claims.Audience, g.oauth.ClientID) || !slices.Contains(googleIssuers, claims.Issuer) {
		return nil, fmt.Errorf("%w: audience or issuer", ErrGoogleSignIn)
	}
	if claims.ExpiresAt == nil || claims.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("%w: expired", ErrGoogleSignIn)
	}
	if claims.Subject == "" || claims.Email == "" || !claims.EmailVerified {
		return nil, fmt.Errorf("%w: unverified email", ErrGoogleSignIn)
	}

	return &GoogleIdentity{
		Subject:   claims.Subject,
		Email:     claims.Email,
		FirstName: claims.GivenName,
		LastName:  claims.FamilyName,
	}, nil
}
