package services

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	tenancyModels "josex/web/modules/tenancy/models"
	"josex/web/modules/tracking/models"
)

// AccessRepository is what the access slice needs from the tracking repository (TRACK-032).
type AccessRepository interface {
	GrantAppAccess(ctx context.Context, tenantID uuid.UUID, dto *models.GrantAccessDto, level string) (*models.AppAccessGrant, error)
	RevokeAppAccess(ctx context.Context, tenantID, userID uuid.UUID, level string) error
	TenantName(ctx context.Context, tenantID uuid.UUID) (string, error)
	UserLinked(ctx context.Context, tenantID, userID uuid.UUID) (bool, error)
	ListRiderGuardians(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID *uuid.UUID) ([]*models.RiderGuardian, error)
	AddRiderGuardian(ctx context.Context, tenantID, riderID uuid.UUID, grant *models.AppAccessGrant, phone *string, scopeUserID *uuid.UUID) error
	RemoveRiderGuardian(ctx context.Context, tenantID, riderID, userID uuid.UUID, scopeUserID *uuid.UUID) error
	SetDriverAccount(ctx context.Context, tenantID, driverID uuid.UUID, userID *uuid.UUID) (*uuid.UUID, error)
	UpsertOrganizationMember(ctx context.Context, tenantID, organizationID, userID uuid.UUID, role string) (*models.OrganizationMember, error)
	DeleteOrganizationMember(ctx context.Context, tenantID, organizationID, userID uuid.UUID) error
}

// AccessService grants and revokes app access from Tracking: the level is fixed by where it is
// granted — a rider's family is portal, a driver driver, an organization member organization.
//
// Every grant is two calls: tenancy's (account + level, main database) then tracking's link. A link
// that fails takes back a membership the grant just added, so a refused rider leaves no stray access.
// Every unlink asks whether anything still names the account and, if not, ends its membership.
type AccessService struct {
	repo  AccessRepository
	email coreServices.EmailService
}

func NewAccessService(repo AccessRepository, email coreServices.EmailService) *AccessService {
	return &AccessService{repo: repo, email: email}
}

func (s *AccessService) ListRiderGuardians(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID *uuid.UUID, lang string) ([]*models.RiderGuardian, error) {
	guardians, err := s.repo.ListRiderGuardians(ctx, tenantID, riderID, scopeUserID)
	if err != nil || len(guardians) == 0 {
		return guardians, err
	}
	tenantName, err := s.repo.TenantName(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, g := range guardians {
		email := ""
		if g.Email != nil {
			email = *g.Email
		}
		g.Notice = accessNotice(lang, g.FirstName, tenantName, email)
	}
	return guardians, nil
}

func (s *AccessService) AddRiderGuardian(ctx context.Context, tenantID, riderID uuid.UUID, dto *models.GrantAccessDto, scopeUserID *uuid.UUID, lang string) (*models.AccessGranted, error) {
	return s.grant(ctx, tenantID, dto, tenancyModels.RolePortal, lang, func(g *models.AppAccessGrant) error {
		return s.repo.AddRiderGuardian(ctx, tenantID, riderID, g, dto.Phone, scopeUserID)
	})
}

func (s *AccessService) RemoveRiderGuardian(ctx context.Context, tenantID, riderID, userID uuid.UUID, scopeUserID *uuid.UUID) error {
	if err := s.repo.RemoveRiderGuardian(ctx, tenantID, riderID, userID, scopeUserID); err != nil {
		return err
	}
	return s.revokeIfUnlinked(ctx, tenantID, userID, tenancyModels.RolePortal)
}

func (s *AccessService) SetDriverAccount(ctx context.Context, tenantID, driverID uuid.UUID, dto *models.GrantAccessDto, lang string) (*models.AccessGranted, error) {
	return s.grant(ctx, tenantID, dto, tenancyModels.RoleDriver, lang, func(g *models.AppAccessGrant) error {
		previous, err := s.repo.SetDriverAccount(ctx, tenantID, driverID, &g.UserID)
		if err != nil || previous == nil {
			return err
		}
		return s.revokeIfUnlinked(ctx, tenantID, *previous, tenancyModels.RoleDriver)
	})
}

func (s *AccessService) ClearDriverAccount(ctx context.Context, tenantID, driverID uuid.UUID) error {
	previous, err := s.repo.SetDriverAccount(ctx, tenantID, driverID, nil)
	if err != nil || previous == nil {
		return err
	}
	return s.revokeIfUnlinked(ctx, tenantID, *previous, tenancyModels.RoleDriver)
}

func (s *AccessService) InviteOrganizationMember(ctx context.Context, tenantID, organizationID uuid.UUID, dto *models.GrantMemberDto, lang string) (*models.AccessGranted, error) {
	return s.grant(ctx, tenantID, &dto.GrantAccessDto, tenancyModels.RoleOrganization, lang, func(g *models.AppAccessGrant) error {
		_, err := s.repo.UpsertOrganizationMember(ctx, tenantID, organizationID, g.UserID, dto.Role)
		return err
	})
}

func (s *AccessService) RemoveOrganizationMember(ctx context.Context, tenantID, organizationID, userID uuid.UUID) error {
	if err := s.repo.DeleteOrganizationMember(ctx, tenantID, organizationID, userID); err != nil {
		return err
	}
	return s.revokeIfUnlinked(ctx, tenantID, userID, tenancyModels.RoleOrganization)
}

// grant runs tenancy's grant, then link; on a failed link it takes back the membership this call
// added. The created account itself stays: with no membership it reaches nothing, and the next grant
// for the same email finds it instead of failing on the unique email.
func (s *AccessService) grant(ctx context.Context, tenantID uuid.UUID, dto *models.GrantAccessDto, level, lang string, link func(*models.AppAccessGrant) error) (*models.AccessGranted, error) {
	g, err := s.repo.GrantAppAccess(ctx, tenantID, dto, level)
	if err != nil {
		return nil, err
	}
	if err := link(g); err != nil {
		if g.MembershipAdded {
			if rerr := s.repo.RevokeAppAccess(ctx, tenantID, g.UserID, level); rerr != nil {
				log.Printf("⚠️  access: could not take back %s membership of %s: %v", level, g.UserID, rerr)
			}
		}
		return nil, err
	}

	notice := accessNotice(lang, g.FirstName, g.TenantName, g.Email)
	if g.MembershipAdded {
		go s.mailNotice(lang, g, notice)
	}
	return &models.AccessGranted{
		UserID: g.UserID, Email: g.Email, FirstName: g.FirstName, LastName: g.LastName,
		Status: g.Status, Notice: notice,
	}, nil
}

// revokeIfUnlinked ends the membership at level once nothing in the tenant names the account.
func (s *AccessService) revokeIfUnlinked(ctx context.Context, tenantID, userID uuid.UUID, level string) error {
	linked, err := s.repo.UserLinked(ctx, tenantID, userID)
	if err != nil || linked {
		return err
	}
	return s.repo.RevokeAppAccess(ctx, tenantID, userID, level)
}

// mailNotice sends the access email once per new membership — a second rider for the same parent
// links silently. Fire and forget, as the user invitation: the notice is also on the screen (D5).
func (s *AccessService) mailNotice(lang string, g *models.AppAccessGrant, notice string) {
	appConf := config.ModularAppConfig.Core
	es := isSpanish(lang)
	subject := fmt.Sprintf("%s gave you access to the %s app", g.TenantName, appConf.AppName)
	button, signOff, automated := "Get the app", fmt.Sprintf("The %s team", appConf.AppName),
		"This is an automated message. Please do not reply."
	if es {
		subject = fmt.Sprintf("%s te dio acceso a la app %s", g.TenantName, appConf.AppName)
		button, signOff, automated = "Obtener la app", fmt.Sprintf("El equipo de %s", appConf.AppName),
			"Este es un mensaje automático. Por favor, no respondas."
	}
	data := map[string]string{
		"Title":         subject,
		"Description":   notice,
		"ButtonText":    button,
		"LandingURL":    appConf.MobileLandingURL,
		"SignOff":       signOff,
		"AutomatedNote": automated,
	}
	templatePath := coreServices.GetTemplatePath("tracking", "access-granted.html")
	if err := s.email.SendEmail(g.Email, subject, templatePath, data); err != nil {
		log.Printf("⚠️  access: could not mail %s: %v", g.Email, err)
	}
}

// accessNotice is the text the email carries and the "Copy message" button copies (D5): who gave the
// access, where the app is, and how to sign in — with the emailed code, never a password (D1).
func accessNotice(lang string, firstName *string, tenantName, email string) string {
	appConf := config.ModularAppConfig.Core
	name := ""
	if firstName != nil {
		name = " " + strings.TrimSpace(*firstName)
	}
	if isSpanish(lang) {
		return fmt.Sprintf("Hola%s: %s te dio acceso a la app %s. Descárgala desde %s y entra con tu correo %s; te enviaremos un código para ingresar.",
			name, tenantName, appConf.AppName, appConf.MobileLandingURL, email)
	}
	return fmt.Sprintf("Hi%s, %s gave you access to the %s app. Get it at %s and sign in with your email %s; we will send you a code to get in.",
		name, tenantName, appConf.AppName, appConf.MobileLandingURL, email)
}

func isSpanish(lang string) bool {
	return strings.HasPrefix(strings.ToLower(lang), "es")
}
