package models

// AvailablePermission represents a permission available to assign to a role
type AvailablePermission struct {
	Code        string `json:"code"`
	Module      string `json:"module"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Section is the row of the roles screen it belongs to; Access says whether "Ver" holds it or only
	// "Editar" does (USERS-015 D2).
	Section string `json:"section"`
	Access  string `json:"access"`
	// AlsoIn names the other sections whose "Ver" needs this read: the API gates trips, the live map and
	// the documents through a read another section owns.
	AlsoIn []string `json:"-"`
}

// The two kinds of access a permission grants.
const (
	AccessRead  = "read"
	AccessWrite = "write"
)

// SectionPermission is one code of a section and the level that holds it.
type SectionPermission struct {
	Code   string `json:"code"`
	Access string `json:"access"`
}

// PermissionSection is one row of the roles screen: "Ver" is its read codes, "Editar" all of them.
type PermissionSection struct {
	Code        string              `json:"code"`
	Module      string              `json:"module"`
	Label       string              `json:"label"`
	Permissions []SectionPermission `json:"permissions"`
}

// PermissionCatalog is GET /permissions.
type PermissionCatalog struct {
	Sections []PermissionSection `json:"sections"`
}
