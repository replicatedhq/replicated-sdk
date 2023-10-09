package types

type ErrorResponse struct {
	Error   string `json:"error,omitempty"`
	Success bool   `json:"success"`
}
