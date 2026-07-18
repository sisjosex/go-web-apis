package models

// AvailablePermission represents a permission available to assign to a role
type AvailablePermission struct {
	Code        string `json:"code"`
	Module      string `json:"module"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
