package models

import "github.com/google/uuid"

// GrantAccessDto is the body of every "give this person the app" endpoint (TRACK-032): an existing
// account by user_id, or a person by email and name, found or created. The level is the endpoint's,
// never the client's.
type GrantAccessDto struct {
	UserID    *uuid.UUID `json:"user_id" binding:"required_without=Email,omitempty,uuidv4"`
	Email     *string    `json:"email" binding:"required_without=UserID,omitempty,email-valid,max=255" conform:"trim,lowercase"`
	FirstName *string    `json:"first_name" binding:"required_with=Email,omitempty,min=1,max=100" conform:"trim"`
	LastName  *string    `json:"last_name" binding:"required_with=Email,omitempty,min=1,max=100" conform:"trim"`
	Phone     *string    `json:"phone" binding:"omitempty,max=50" conform:"trim"`
}

// GrantMemberDto adds the organization role a member holds (TRACK-005 D3).
type GrantMemberDto struct {
	GrantAccessDto
	Role string `json:"role" binding:"required,oneof=admin viewer supervisor" conform:"trim,lowercase"`
}

// AppAccessGrant is what tenancy.sp_grant_app_access answers: the account, whether it was created
// ("new") or already there ("linked"), and whether this call added its membership — which is the one
// a failed link must take back.
type AppAccessGrant struct {
	UserID          uuid.UUID
	Email           string
	FirstName       *string
	LastName        *string
	Status          string
	MembershipAdded bool
	TenantName      string
}

// AccessGranted is the 201 of a grant. Notice is the text the access email carries, for the
// "Copy message" button (D5).
type AccessGranted struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	FirstName *string   `json:"first_name"`
	LastName  *string   `json:"last_name"`
	Status    string    `json:"status"`
	Notice    string    `json:"notice"`
}

// RiderGuardian is one row of a rider's Family tab (D3).
type RiderGuardian struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     *string   `json:"email"`
	FirstName *string   `json:"first_name"`
	LastName  *string   `json:"last_name"`
	Phone     *string   `json:"phone"`
	IsPrimary bool      `json:"is_primary"`
	Notice    string    `json:"notice"`
}
