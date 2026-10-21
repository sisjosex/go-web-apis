package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
)

// Invitations at scale (TRACK-062): the same grant TRACK-032 gives one guardian, run for a whole
// organization or a selection of riders, and each account's invitation status beside it.

// skipReasons are the grant refusals a bulk invite reports per contact instead of failing.
var skipReasons = map[string]string{
	trackingErrors.AccessWebAccount:    models.InviteWebAccount,
	trackingErrors.AccessLevelMismatch: models.InviteLevelMismatch,
	trackingErrors.AccessUserDeleted:   models.InviteUserDeleted,
}

// Invite gives every guardian contact without an account of the organization or the riders the app:
// an account by its email (D1: a phone alone is skipped), the portal level, the link onto the same
// contact row and the access email. One person guarding siblings is granted once and linked to each.
// A contact with a phone is meant to get the message by WhatsApp from the operator (D2).
func (s *AccessService) Invite(ctx context.Context, tenantID uuid.UUID, dto *models.InviteDto, scopeUserID, actingUserID *uuid.UUID, lang string) (*models.InviteResult, error) {
	contacts, err := s.repo.ListInvitableContacts(ctx, tenantID, dto.OrganizationID, dto.RiderIDs, scopeUserID)
	if err != nil {
		return nil, err
	}
	run := &inviteRun{s: s, tenantID: tenantID, scopeUserID: scopeUserID, actingUserID: actingUserID,
		grants: map[string]*models.AppAccessGrant{}, refused: map[string]string{}}
	result := &models.InviteResult{Skipped: []models.InviteSkipped{}, Items: []models.InviteItem{}}
	for _, c := range contacts {
		reason, err := run.contact(ctx, c)
		if err != nil {
			return nil, err
		}
		if reason != "" {
			result.Skipped = append(result.Skipped, models.InviteSkipped{ContactID: c.ContactID, RiderName: c.RiderName, Reason: reason})
		}
	}
	order := run.order
	if len(order) == 0 {
		return result, nil
	}

	ids := make([]uuid.UUID, len(order))
	for i, g := range order {
		ids[i] = g.UserID
	}
	invites, err := s.invitesByUser(ctx, tenantID, ids, "")
	if err != nil {
		return nil, err
	}
	for _, g := range order {
		notice := accessNotice(lang, g.FirstName, g.TenantName, g.Email)
		if g.MembershipAdded {
			go s.mailNotice(lang, g, notice)
		}
		item := models.InviteItem{UserID: g.UserID, Channel: models.InviteEmail, Status: "pending", Message: notice}
		if inv, ok := invites[g.UserID]; ok {
			item.Channel, item.Status = inv.Channel, inv.Status
		}
		result.Items = append(result.Items, item)
	}
	result.Invited = len(result.Items)
	return result, nil
}

// inviteRun is one bulk invite's state: each person granted once, by email, in the order reached.
type inviteRun struct {
	s                         *AccessService
	tenantID                  uuid.UUID
	scopeUserID, actingUserID *uuid.UUID
	grants                    map[string]*models.AppAccessGrant
	refused                   map[string]string
	order                     []*models.AppAccessGrant
}

// contact grants the contact's person (once) and links the account onto the contact's rider, or
// answers why the contact is skipped.
func (r *inviteRun) contact(ctx context.Context, c models.InvitableContact) (string, error) {
	if c.Email == nil {
		if c.Phone != nil {
			return models.InviteNoEmail, nil
		}
		return models.InviteNoDestination, nil
	}
	email := *c.Email
	if reason, ok := r.refused[email]; ok {
		return reason, nil
	}
	g, ok := r.grants[email]
	if !ok {
		var err error
		if g, err = r.grant(ctx, c); err != nil {
			var trackingErr *trackingErrors.TrackingError
			if errors.As(err, &trackingErr) && skipReasons[trackingErr.Code] != "" {
				r.refused[email] = skipReasons[trackingErr.Code]
				return r.refused[email], nil
			}
			return "", err
		}
		r.grants[email] = g
		r.order = append(r.order, g)
	}
	return "", r.s.repo.AddRiderGuardian(ctx, r.tenantID, c.RiderID, &g.UserID, deref(c.Name), email, c.Phone, r.scopeUserID)
}

// grant gives the contact's person the portal level; one with a phone is meant to get the message by
// WhatsApp from the operator (D2).
func (r *inviteRun) grant(ctx context.Context, c models.InvitableContact) (*models.AppAccessGrant, error) {
	channel := models.InviteEmail
	if c.Phone != nil {
		channel = models.InviteWhatsApp
	}
	first, last := splitName(c.Name)
	return r.s.repo.GrantAppAccess(ctx, r.tenantID, &models.GrantAccessDto{
		Email: c.Email, FirstName: first, LastName: last, Phone: c.Phone,
	}, tenancyModels.RolePortal, channel, r.actingUserID)
}

// ListOrganizationInvites is the organization's "Invitaciones" tab: its riders' guardian accounts with
// their invitation. A status filter reads the matching invitations from the main database first, so
// the page and its total are of those accounts only.
func (s *AccessService) ListOrganizationInvites(ctx context.Context, tenantID, organizationID uuid.UUID, query models.ListOrganizationInvitesQuery, scopeUserID *uuid.UUID, lang string) (*models.ListOrganizationInvitesResponse, error) {
	var only []uuid.UUID
	var invites map[uuid.UUID]models.AppInvite
	if query.Status != "" {
		var err error
		if invites, err = s.invitesByUser(ctx, tenantID, nil, query.Status); err != nil {
			return nil, err
		}
		only = make([]uuid.UUID, 0, len(invites))
		for id := range invites {
			only = append(only, id)
		}
	}
	rows, total, err := s.repo.ListOrganizationInvited(ctx, tenantID, organizationID, scopeUserID, only, query.Page, query.PageSize)
	if err != nil {
		return nil, err
	}
	response := &models.ListOrganizationInvitesResponse{Invites: rows, TotalCount: total, Page: query.Page, PageSize: query.PageSize}
	if len(rows) == 0 {
		return response, nil
	}
	if invites == nil {
		ids := make([]uuid.UUID, len(rows))
		for i, row := range rows {
			ids[i] = row.UserID
		}
		if invites, err = s.invitesByUser(ctx, tenantID, ids, ""); err != nil {
			return nil, err
		}
	}
	tenantName, err := s.repo.TenantName(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if inv, ok := invites[row.UserID]; ok {
			row.Channel, row.Status, row.SentAt = &inv.Channel, &inv.Status, inv.SentAt
		}
		row.Message = accessNotice(lang, row.FirstName, tenantName, deref(row.Email))
	}
	return response, nil
}

// MarkInviteSent records that the operator sent the guardian's invitation (by WhatsApp, D2).
func (s *AccessService) MarkInviteSent(ctx context.Context, tenantID, userID uuid.UUID, scopeUserID *uuid.UUID) error {
	if _, err := s.repo.InvitedGuardian(ctx, tenantID, userID, scopeUserID); err != nil {
		return err
	}
	_, err := s.repo.MarkAppInviteSent(ctx, tenantID, userID)
	return err
}

// ResendInvite answers the invitation's message again; an email invitation is mailed again and
// counts as sent now. A WhatsApp one is sent by the operator, who then marks it sent.
func (s *AccessService) ResendInvite(ctx context.Context, tenantID, userID uuid.UUID, scopeUserID *uuid.UUID, lang string) (*models.InviteMessage, error) {
	guardian, err := s.repo.InvitedGuardian(ctx, tenantID, userID, scopeUserID)
	if err != nil {
		return nil, err
	}
	invites, err := s.invitesByUser(ctx, tenantID, []uuid.UUID{userID}, "")
	if err != nil {
		return nil, err
	}
	inv, ok := invites[userID]
	if !ok {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.InviteNotFound}
	}
	tenantName, err := s.repo.TenantName(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	email := deref(guardian.Email)
	notice := accessNotice(lang, guardian.FirstName, tenantName, email)
	if inv.Channel == models.InviteEmail && email != "" {
		if _, err := s.repo.MarkAppInviteSent(ctx, tenantID, userID); err != nil {
			return nil, err
		}
		go s.mailNotice(lang, &models.AppAccessGrant{UserID: userID, Email: email, FirstName: guardian.FirstName, TenantName: tenantName}, notice)
	}
	return &models.InviteMessage{Message: notice}, nil
}

// invitesByUser reads the business's invitations of ids (nil: any) in status ("": any), by account.
func (s *AccessService) invitesByUser(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID, status string) (map[uuid.UUID]models.AppInvite, error) {
	list, err := s.repo.ListAppInvites(ctx, tenantID, ids, status)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]models.AppInvite, len(list))
	for _, inv := range list {
		out[inv.UserID] = inv
	}
	return out, nil
}

// attachInvites puts each guardian account's invitation on its family row (TRACK-062).
func (s *AccessService) attachInvites(ctx context.Context, tenantID uuid.UUID, guardians []*models.RiderGuardian) error {
	ids := make([]uuid.UUID, 0, len(guardians))
	for _, g := range guardians {
		if g.UserID != nil {
			ids = append(ids, *g.UserID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	invites, err := s.invitesByUser(ctx, tenantID, ids, "")
	if err != nil {
		return err
	}
	for _, g := range guardians {
		if g.UserID == nil {
			continue
		}
		if inv, ok := invites[*g.UserID]; ok {
			g.Invite = &models.AppInvite{Channel: inv.Channel, Status: inv.Status, SentAt: inv.SentAt}
		}
	}
	return nil
}

// splitName turns a contact's one name into the account's first and last: the first word, then the rest.
func splitName(name *string) (*string, *string) {
	if blank(name) {
		return nil, nil
	}
	parts := strings.Fields(*name)
	first := parts[0]
	if len(parts) == 1 {
		return &first, nil
	}
	last := strings.Join(parts[1:], " ")
	return &first, &last
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
