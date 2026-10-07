package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
)

// AccessRepository is what the access slice needs from the tracking repository (TRACK-032).
type AccessRepository interface {
	GrantAppAccess(ctx context.Context, tenantID uuid.UUID, dto *models.GrantAccessDto, level string) (*models.AppAccessGrant, error)
	CreateRider(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error)
	CreateDriver(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDriverDto) (*models.Driver, error)
	RevokeAppAccess(ctx context.Context, tenantID, userID uuid.UUID, level string) error
	TenantName(ctx context.Context, tenantID uuid.UUID) (string, error)
	UserLinked(ctx context.Context, tenantID, userID uuid.UUID) (bool, error)
	ListRiderGuardians(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID *uuid.UUID) ([]*models.RiderGuardian, error)
	AddRiderGuardian(ctx context.Context, tenantID, riderID uuid.UUID, userID *uuid.UUID, name, email string, phone *string, scopeUserID *uuid.UUID) error
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

// AddRiderGuardian takes one guardian as the rider form does (TRACK-048 D3): an account by user_id, a
// person to invite by email and name, or a contact with only a name and a phone, who gets no account.
func (s *AccessService) AddRiderGuardian(ctx context.Context, tenantID, riderID uuid.UUID, dto *models.NewRiderGuardianDto, scopeUserID *uuid.UUID, lang string) (*models.AccessGranted, error) {
	if dto.UserID == nil && dto.Email == nil {
		if blank(dto.FirstName) || blank(dto.Phone) {
			return nil, &trackingErrors.TrackingError{Code: trackingErrors.RiderGuardianInvalid}
		}
		if err := s.repo.AddRiderGuardian(ctx, tenantID, riderID, nil, joinNames(dto.FirstName, dto.LastName), "", dto.Phone, scopeUserID); err != nil {
			return nil, err
		}
		return &models.AccessGranted{FirstName: dto.FirstName, LastName: dto.LastName, Status: models.GuardianContact}, nil
	}
	if dto.UserID == nil && (blank(dto.FirstName) || blank(dto.LastName)) {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RiderGuardianInvalid}
	}
	grant := &models.GrantAccessDto{UserID: dto.UserID, Email: dto.Email, FirstName: dto.FirstName, LastName: dto.LastName, Phone: dto.Phone}
	return s.grant(ctx, tenantID, grant, tenancyModels.RolePortal, lang, func(g *models.AppAccessGrant) error {
		return s.repo.AddRiderGuardian(ctx, tenantID, riderID, &g.UserID, joinNames(g.FirstName, g.LastName), g.Email, dto.Phone, scopeUserID)
	})
}

// CreateRider creates a rider with its guardians in one request (TRACK-037 D2). The accounts are
// granted first, in the main database; the rider and every link are then one statement in the
// tenant's. A refused item names its index, and a failed create takes back every membership this call
// added, so nothing is left half-made. The access emails go out only once the rider exists.
func (s *AccessService) CreateRider(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRiderDto, scopeUserID *uuid.UUID, lang string) (*models.CreatedRider, error) {
	grants := make([]*models.AppAccessGrant, 0, len(dto.Guardians))
	takeBack := func() {
		for _, g := range grants {
			if g.MembershipAdded {
				if err := s.repo.RevokeAppAccess(ctx, tenantID, g.UserID, tenancyModels.RolePortal); err != nil {
					log.Printf("⚠️  access: could not take back portal membership of %s: %v", g.UserID, err)
				}
			}
		}
	}

	links := make([]map[string]any, 0, len(dto.Guardians))
	for i := range dto.Guardians {
		link, g, err := s.guardianLink(ctx, tenantID, i, &dto.Guardians[i])
		if err != nil {
			takeBack()
			return nil, err
		}
		if g != nil {
			grants = append(grants, g)
		}
		links = append(links, link)
	}

	raw, err := json.Marshal(links)
	if err != nil {
		takeBack()
		return nil, err
	}
	dto.LinkedGuardians = raw
	rider, err := s.repo.CreateRider(ctx, tenantID, dto, scopeUserID)
	if err != nil {
		takeBack()
		return nil, err
	}

	granted := make([]*models.AccessGranted, 0, len(grants))
	for _, g := range grants {
		notice := accessNotice(lang, g.FirstName, g.TenantName, g.Email)
		if g.MembershipAdded {
			go s.mailNotice(lang, g, notice)
		}
		granted = append(granted, &models.AccessGranted{
			UserID: &g.UserID, Email: g.Email, FirstName: g.FirstName, LastName: g.LastName, Status: g.Status, Notice: notice,
		})
	}
	return &models.CreatedRider{Rider: rider, Guardians: granted}, nil
}

// CreateDriver creates a driver and, when asked, their app access in one request (TRACK-047 D3): the
// account is granted the driver level first, under the driver's own name, then the driver is written
// already linked to it. A refused write takes back the membership this call added.
func (s *AccessService) CreateDriver(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDriverDto, lang string) (*models.CreatedDriver, error) {
	if dto.Account == nil {
		driver, err := s.repo.CreateDriver(ctx, tenantID, dto)
		if err != nil {
			return nil, err
		}
		return &models.CreatedDriver{Driver: driver}, nil
	}

	phone := dto.Account.Phone
	if phone == nil {
		phone = dto.Phone
	}
	grant := &models.GrantAccessDto{Email: &dto.Account.Email, FirstName: &dto.FirstName, LastName: &dto.LastName, Phone: phone}
	var driver *models.Driver
	granted, err := s.grant(ctx, tenantID, grant, tenancyModels.RoleDriver, lang, func(g *models.AppAccessGrant) error {
		dto.UserID = &g.UserID
		var err error
		driver, err = s.repo.CreateDriver(ctx, tenantID, dto)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &models.CreatedDriver{Driver: driver, Account: granted}, nil
}

// guardianLink is one guardian as fn_rider_guardians_link reads it. A contact (no account, no email)
// needs a name to show and a phone to call (D3); anyone else is granted the portal level first and
// comes back with the grant a failed create must take back.
func (s *AccessService) guardianLink(ctx context.Context, tenantID uuid.UUID, index int, item *models.NewRiderGuardianDto) (map[string]any, *models.AppAccessGrant, error) {
	if item.UserID == nil && item.Email == nil {
		if blank(item.FirstName) || blank(item.Phone) {
			return nil, nil, guardianInvalid(index, nil)
		}
		return map[string]any{"name": joinNames(item.FirstName, item.LastName), "phone": item.Phone}, nil, nil
	}
	if item.UserID == nil && (blank(item.FirstName) || blank(item.LastName)) {
		return nil, nil, guardianInvalid(index, nil)
	}
	g, err := s.repo.GrantAppAccess(ctx, tenantID, &models.GrantAccessDto{
		UserID: item.UserID, Email: item.Email, FirstName: item.FirstName, LastName: item.LastName, Phone: item.Phone,
	}, tenancyModels.RolePortal)
	if err != nil {
		return nil, nil, guardianInvalid(index, err)
	}
	return map[string]any{
		"user_id": g.UserID, "name": joinNames(g.FirstName, g.LastName), "email": g.Email, "phone": item.Phone,
	}, g, nil
}

// guardianInvalid names the refused guardian. A grant's own refusal (a web account, another app
// level) keeps its code so the form can say why; anything else is tracking.rider.guardian-invalid.
func guardianInvalid(index int, err error) error {
	detail := map[string]int{"index": index}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) && trackingErr.Code != trackingErrors.AccessGrantFailed {
		return &trackingErrors.TrackingError{Code: trackingErr.Code, Err: err, Detail: detail}
	}
	return &trackingErrors.TrackingError{Code: trackingErrors.RiderGuardianInvalid, Err: err, Detail: detail}
}

func blank(value *string) bool {
	return value == nil || strings.TrimSpace(*value) == ""
}

func joinNames(first, last *string) string {
	parts := make([]string, 0, 2)
	for _, p := range []*string{first, last} {
		if !blank(p) {
			parts = append(parts, strings.TrimSpace(*p))
		}
	}
	return strings.Join(parts, " ")
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
		UserID: &g.UserID, Email: g.Email, FirstName: g.FirstName, LastName: g.LastName,
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
