package models

type CreateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateUserRoleRequest struct {
	RoleID string `json:"role_id"`
}
