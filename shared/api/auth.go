package api

// Request DTOs

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type CheckConfirmationCodeRequest struct {
	Email            string `json:"email" validate:"required,email"`
	ConfirmationCode string `json:"confirmation_code" validate:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Response DTOs
// Note: Auth responses are currently plain messages, not domain types
// If you add user data to login response, embed domain.User here

type RegisterResponse struct {
	Message string `json:"message"`
}

type LoginResponse struct {
	Message     string `json:"message"`
	AccessToken string `json:"access_token,omitempty"` // Token for non-cookie clients (mobile, API clients)
}

type LogoutResponse struct {
	Message string `json:"message"`
}
