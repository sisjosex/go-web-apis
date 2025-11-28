package models

// UserListQuery represents query parameters for listing users
type UserListQuery struct {
	Page   int    `form:"page" json:"page" binding:"omitempty,min=1"`                                       // Page number (default: 1)
	Limit  int    `form:"limit" json:"limit" binding:"omitempty,min=1,max=100"`                             // Items per page (default: 10, max: 100)
	Search string `form:"search" json:"search"`                                                             // Search in name/email
	Status string `form:"status" json:"status" binding:"omitempty,oneof=active inactive expired"`           // Filter by status
	Sort   string `form:"sort" json:"sort" binding:"omitempty,oneof=created_at email first_name last_name"` // Sort field
	Order  string `form:"order" json:"order" binding:"omitempty,oneof=asc desc"`                            // Sort order (asc/desc)
}

// UserListResponse represents paginated user list response
type UserListResponse struct {
	Users      []User `json:"users"`
	Total      int64  `json:"total"`       // Total count of users
	Page       int    `json:"page"`        // Current page
	Limit      int    `json:"limit"`       // Items per page
	TotalPages int    `json:"total_pages"` // Total pages
}

// SoftDeleteUserDto represents soft delete request
type SoftDeleteUserDto struct {
	ID     string `json:"id" binding:"required,uuidv4"`
	Reason string `json:"reason" binding:"omitempty,max=500"` // Optional deletion reason
}
