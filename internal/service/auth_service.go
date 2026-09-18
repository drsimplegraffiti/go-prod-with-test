package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/example/goapi/internal/email"
	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
	"github.com/example/goapi/internal/utils"
)

// ErrInvalidCredentials is returned for any login failure. It is
// intentionally generic (never "email not found" vs "wrong password") to
// avoid leaking which emails are registered.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrEmailTaken is returned when registering with an email already in use.
var ErrEmailTaken = errors.New("email is already registered")

// AuthService implements registration, login and token refresh.
type AuthService struct {
	users      *repository.UserRepository
	jwt        *utils.JWTManager
	email      *email.Service
	bcryptCost int
}

// NewAuthService constructs an AuthService.
func NewAuthService(users *repository.UserRepository, jwt *utils.JWTManager,
	emailService *email.Service,
	bcryptCost int,
) *AuthService {
	return &AuthService{
		users: users, jwt: jwt,
		email:      emailService,
		bcryptCost: bcryptCost,
	}
}

// Register creates a new user account and returns freshly issued tokens.
func (s *AuthService) Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
	email := normalizeEmail(req.Email)
	if err := validateRegisterRequest(email, req.Password, req.Name); err != nil {
		return nil, err
	}

	if _, err := s.users.GetByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	}

	hash, err := utils.HashPassword(req.Password, s.bcryptCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:        email,
		PasswordHash: hash,
		Name:         strings.TrimSpace(req.Name),
		Role:         models.RoleUser,
	}

	created, err := s.users.Create(ctx, user)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	s.email.SendWelcomeEmail(
		created.Email,
		created.Name,
	)

	return s.issueTokens(created)
}

func (s *AuthService) LoginAttempt(ctx context.Context, ip string) error {
	slog.Info(
		"login attempt",
		"ip", ip,
	)

	return nil
}

// Login verifies credentials and issues new tokens on success.
func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	email := normalizeEmail(req.Email)

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokens(user)
}

// Refresh validates a refresh token and issues a new token pair
// (rotation: the old refresh token is not tracked/blacklisted here, but the
// interface makes it straightforward to add a token-store check later).
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
	claims, err := s.jwt.Verify(refreshToken, utils.RefreshToken)
	if err != nil {
		return nil, utils.ErrInvalidToken
	}

	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, utils.ErrInvalidToken
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	return s.issueTokens(user)
}

func (s *AuthService) issueTokens(user *models.User) (*models.AuthResponse, error) {
	access, expiresAt, err := s.jwt.GenerateAccessToken(user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}
	refresh, _, err := s.jwt.GenerateRefreshToken(user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	return &models.AuthResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(expiresAt.Unix()),
		User:         *user,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateRegisterRequest(email, password, name string) error {
	if email == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("%w: a valid email is required", ErrValidation)
	}
	if len(password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrValidation)
	}
	if len(password) > 72 {
		return fmt.Errorf("%w: password must be at most 72 characters", ErrValidation)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	return nil
}
