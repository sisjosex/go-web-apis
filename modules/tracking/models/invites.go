package models

import (
	"time"

	"github.com/google/uuid"
)

// Invitations at scale (TRACK-062): the guardians of an organization or of a selection of riders get
// the app in one action, and each account's invitation says where it stands.

// Why a contact was left out of a bulk invite. D1: only a contact with an email gets an account — a
// phone alone cannot receive the sign-in code.
const (
	InviteNoDestination = "no-destination"
	InviteNoEmail       = "no-email"
	InviteWebAccount    = "web-account"
	InviteLevelMismatch = "level-mismatch"
	InviteUserDeleted   = "user-deleted"
)

// InviteDto is the body of POST /tracking/invites: every family of an organization, or of the
// selected riders.
type InviteDto struct {
	OrganizationID *uuid.UUID  `json:"organization_id" binding:"required_without=RiderIDs,omitempty"`
	RiderIDs       []uuid.UUID `json:"rider_ids" binding:"required_without=OrganizationID,omitempty,min=1,max=500"`
}

// InvitableContact is a guardian contact without an account, as sp_list_invitable_contacts answers it.
type InvitableContact struct {
	ContactID uuid.UUID
	RiderID   uuid.UUID
	RiderName string
	Name      *string
	Phone     *string
	Email     *string
}

// InviteSkipped is a contact the bulk invite left out, and why.
type InviteSkipped struct {
	ContactID uuid.UUID `json:"contact_id"`
	RiderName string    `json:"rider_name"`
	Reason    string    `json:"reason"`
}

// InviteItem is one account the bulk invite reached; Message is the text to send by WhatsApp or copy.
type InviteItem struct {
	UserID  uuid.UUID `json:"user_id"`
	Channel string    `json:"channel"`
	Status  string    `json:"status"`
	Message string    `json:"message"`
}

// InviteResult is the 200 of POST /tracking/invites.
type InviteResult struct {
	Invited int             `json:"invited"`
	Skipped []InviteSkipped `json:"skipped"`
	Items   []InviteItem    `json:"items"`
}

// AppInvite is an account's invitation in the business, as tenancy.sp_list_app_invites derives it:
// status pending (not sent yet), sent, accepted (signed in since) or expired (sent over 30 days ago).
type AppInvite struct {
	UserID  uuid.UUID  `json:"-"`
	Channel string     `json:"channel"`
	Status  string     `json:"status"`
	SentAt  *time.Time `json:"sent_at"`
}

// OrganizationInvite is a row of an organization's "Invitaciones" tab: a guardian account of its
// riders and its invitation, nil fields when the account has none.
type OrganizationInvite struct {
	UserID    uuid.UUID  `json:"user_id"`
	FirstName *string    `json:"first_name"`
	LastName  *string    `json:"last_name"`
	Phone     *string    `json:"phone"`
	Email     *string    `json:"email"`
	Riders    []string   `json:"riders"`
	Channel   *string    `json:"channel"`
	Status    *string    `json:"status"`
	SentAt    *time.Time `json:"sent_at"`
	Message   string     `json:"message"`
}

// ListOrganizationInvitesQuery binds GET /tracking/organizations/:id/invites.
type ListOrganizationInvitesQuery struct {
	Status   string `form:"status" binding:"omitempty,oneof=pending sent accepted expired"`
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
}

type ListOrganizationInvitesResponse struct {
	Invites    []*OrganizationInvite `json:"invites"`
	TotalCount int64                 `json:"total_count"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
}

// InvitedGuardian is the account an invitation is sent or resent to.
type InvitedGuardian struct {
	UserID    uuid.UUID
	FirstName *string
	Email     *string
	Phone     *string
}

// InviteMessage is the 200 of a resend: the text to send again.
type InviteMessage struct {
	Message string `json:"message"`
}
