package api

// Request DTOs

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RegisterWithInviteRequest struct {
	InviteCode string `json:"invite_code" validate:"required"`
	Password   string `json:"password" validate:"required"`
	RefSource  string `json:"ref_source"`
}

type CheckConfirmationCodeRequest struct {
	Email            string `json:"email" validate:"required,email"`
	ConfirmationCode string `json:"confirmation_code" validate:"required"`
	RefSource        string `json:"ref_source"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Response DTOs

type LoginResponse struct {
	Message     string `json:"message"`
	AccessToken string `json:"access_token,omitempty"` // Token for non-cookie clients (mobile, API clients)
}

type RegisterWithInviteResponse struct {
	Message string `json:"message"`
	Email   string `json:"email"`
}
