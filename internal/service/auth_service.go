// package service
//
// import (
// 	"context"
// 	"errors"
// 	"fmt"
// 	"log/slog"
// 	"strings"
//
// 	"github.com/example/goapi/internal/email"
// 	"github.com/example/goapi/internal/models"
// 	"github.com/example/goapi/internal/repository"
// 	"github.com/example/goapi/internal/utils"
// )
//
// // ErrInvalidCredentials is returned for any login failure. It is
// // intentionally generic (never "email not found" vs "wrong password") to
// // avoid leaking which emails are registered.
// var ErrInvalidCredentials = errors.New("invalid email or password")
//
// // ErrEmailTaken is returned when registering with an email already in use.
// var ErrEmailTaken = errors.New("email is already registered")
//
// // AuthService implements registration, login and token refresh.
// type AuthService struct {
// 	users      *repository.UserRepository
// 	jwt        *utils.JWTManager
// 	email      *email.Service
// 	bcryptCost int
// }
//
// // NewAuthService constructs an AuthService.
// func NewAuthService(users *repository.UserRepository, jwt *utils.JWTManager,
// 	emailService *email.Service,
// 	bcryptCost int,
// ) *AuthService {
// 	return &AuthService{
// 		users: users, jwt: jwt,
// 		email:      emailService,
// 		bcryptCost: bcryptCost,
// 	}
// }
//
// // Register creates a new user account and returns freshly issued tokens.
// func (s *AuthService) Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
// 	email := normalizeEmail(req.Email)
// 	if err := validateRegisterRequest(email, req.Password, req.Name); err != nil {
// 		return nil, err
// 	}
//
// 	if _, err := s.users.GetByEmail(ctx, email); err == nil {
// 		return nil, ErrEmailTaken
// 	}
//
// 	hash, err := utils.HashPassword(req.Password, s.bcryptCost)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	user := &models.User{
// 		Email:        email,
// 		PasswordHash: hash,
// 		Name:         strings.TrimSpace(req.Name),
// 		Role:         models.RoleUser,
// 	}
//
// 	created, err := s.users.Create(ctx, user)
// 	if err != nil {
// 		if errors.Is(err, repository.ErrDuplicate) {
// 			return nil, ErrEmailTaken
// 		}
// 		return nil, fmt.Errorf("create user: %w", err)
// 	}
//
// 	s.email.SendWelcomeEmail(
// 		created.Email,
// 		created.Name,
// 	)
//
// 	return s.issueTokens(created)
// }
//
// func (s *AuthService) LoginAttempt(ctx context.Context, ip string) error {
// 	slog.Info(
// 		"login attempt",
// 		"ip", ip,
// 	)
//
// 	return nil
// }
//
// // Login verifies credentials and issues new tokens on success.
// func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
// 	email := normalizeEmail(req.Email)
//
// 	user, err := s.users.GetByEmail(ctx, email)
// 	if err != nil {
// 		if errors.Is(err, repository.ErrNotFound) {
// 			return nil, ErrInvalidCredentials
// 		}
// 		return nil, fmt.Errorf("lookup user: %w", err)
// 	}
//
// 	if !utils.CheckPassword(user.PasswordHash, req.Password) {
// 		return nil, ErrInvalidCredentials
// 	}
//
// 	return s.issueTokens(user)
// }
//
// // Refresh validates a refresh token and issues a new token pair
// // (rotation: the old refresh token is not tracked/blacklisted here, but the
// // interface makes it straightforward to add a token-store check later).
// func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
// 	claims, err := s.jwt.Verify(refreshToken, utils.RefreshToken)
// 	if err != nil {
// 		return nil, utils.ErrInvalidToken
// 	}
//
// 	user, err := s.users.GetByID(ctx, claims.UserID)
// 	if err != nil {
// 		if errors.Is(err, repository.ErrNotFound) {
// 			return nil, utils.ErrInvalidToken
// 		}
// 		return nil, fmt.Errorf("lookup user: %w", err)
// 	}
//
// 	return s.issueTokens(user)
// }
//
// func (s *AuthService) issueTokens(user *models.User) (*models.AuthResponse, error) {
// 	access, expiresAt, err := s.jwt.GenerateAccessToken(user.ID, string(user.Role))
// 	if err != nil {
// 		return nil, fmt.Errorf("generate access token: %w", err)
// 	}
// 	refresh, _, err := s.jwt.GenerateRefreshToken(user.ID, string(user.Role))
// 	if err != nil {
// 		return nil, fmt.Errorf("generate refresh token: %w", err)
// 	}
//
// 	return &models.AuthResponse{
// 		AccessToken:  access,
// 		RefreshToken: refresh,
// 		TokenType:    "Bearer",
// 		ExpiresIn:    int64(expiresAt.Unix()),
// 		User:         *user,
// 	}, nil
// }
//
// func normalizeEmail(email string) string {
// 	return strings.ToLower(strings.TrimSpace(email))
// }
//
// func validateRegisterRequest(email, password, name string) error {
// 	if email == "" || !strings.Contains(email, "@") {
// 		return fmt.Errorf("%w: a valid email is required", ErrValidation)
// 	}
// 	if len(password) < 8 {
// 		return fmt.Errorf("%w: password must be at least 8 characters", ErrValidation)
// 	}
// 	if len(password) > 72 {
// 		return fmt.Errorf("%w: password must be at most 72 characters", ErrValidation)
// 	}
// 	if strings.TrimSpace(name) == "" {
// 		return fmt.Errorf("%w: name is required", ErrValidation)
// 	}
// 	return nil
// }

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/example/goapi/internal/email"
	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
	"github.com/example/goapi/internal/utils"
)

// ErrInvalidCredentials is returned for any login failure. It is
// intentionally generic (never "email not found" vs "wrong password") to
// avoid leaking which emails are registered. The not-found path also runs a
// dummy bcrypt comparison so the two paths take equal time.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrEmailTaken is returned when registering with an email already in use.
var ErrEmailTaken = errors.New("email is already registered")

// ErrValidation wraps all input-validation failures.
// var ErrValidation = errors.New("validation error")

// AuthService implements registration, login and token refresh.
type AuthService struct {
	users      *repository.UserRepository
	tokens     *repository.RefreshTokenRepository
	jwt        *utils.JWTManager
	email      *email.Service
	bcryptCost int
}

// NewAuthService constructs an AuthService.
func NewAuthService(users *repository.UserRepository, tokens *repository.RefreshTokenRepository,
	jwt *utils.JWTManager, emailService *email.Service, bcryptCost int,
) *AuthService {
	return &AuthService{
		users:      users,
		tokens:     tokens,
		jwt:        jwt,
		email:      emailService,
		bcryptCost: bcryptCost,
	}
}

// Register creates a new user account and returns freshly issued tokens.
func (s *AuthService) Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("%w: a valid email is required", ErrValidation)
	}
	if err := validateRegisterRequest(req.Password, req.Name); err != nil {
		return nil, err
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
		// Race-safe duplicate handling: no pre-check, we rely on the unique
		// constraint and translate its violation. Two concurrent registrations
		// with the same email cannot both succeed.
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	s.email.SendWelcomeEmail(created.Email, created.Name)

	return s.issueTokens(ctx, created)
}

// Login verifies credentials and issues new tokens on success.
func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		// Malformed email still goes through the timing-equalizing path.
		_ = utils.CheckPassword(utils.DummyHash, req.Password)
		return nil, ErrInvalidCredentials
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			_ = utils.CheckPassword(utils.DummyHash, req.Password)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokens(ctx, user)
}

// Refresh validates a refresh token, revokes it, and issues a rotated pair.
// Reusing an already-rotated (or revoked) token is rejected — if that
// happens, the token may have been stolen, so all of the user's tokens are
// revoked as a defensive measure.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
	claims, err := s.jwt.Verify(refreshToken, utils.RefreshToken)
	if err != nil {
		return nil, utils.ErrInvalidToken
	}

	presentedHash := hashToken(refreshToken)

	rec, err := s.tokens.FindValid(ctx, presentedHash)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Unknown/revoked/expired token presented but with a VALID JWT
			// signature: it was valid once. Possible theft — nuke the session.
			s.tokens.RevokeAllForUser(ctx, claims.UserID)
			return nil, utils.ErrInvalidToken
		}
		return nil, fmt.Errorf("lookup refresh token: %w", err)
	}

	// Rotate: the old token dies here.
	if err := s.tokens.Revoke(ctx, presentedHash); err != nil {
		return nil, fmt.Errorf("revoke refresh token: %w", err)
	}

	user, err := s.users.GetByID(ctx, rec.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, utils.ErrInvalidToken
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	return s.issueTokens(ctx, user)
}

// issueTokens generates a fresh access+refresh pair and persists the refresh
// token hash so Refresh can rotate and revoke it.
func (s *AuthService) issueTokens(ctx context.Context, user *models.User) (*models.AuthResponse, error) {
	access, accessExpiresAt, err := s.jwt.GenerateAccessToken(user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}
	refresh, refreshExpiresAt, err := s.jwt.GenerateRefreshToken(user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	if err := s.tokens.Create(ctx, user.ID, hashToken(refresh), refreshExpiresAt); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &models.AuthResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(accessExpiresAt).Seconds()),
		User:         *user,
	}, nil
}

// hashToken returns the SHA-256 hex digest of a token. Only digests are
// stored, so a database leak does not leak usable refresh tokens.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	return strings.ToLower(addr.Address), nil
}

func validateRegisterRequest(password, name string) error {
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
