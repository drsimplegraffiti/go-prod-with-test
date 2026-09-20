// package service
//
// import (
// 	"context"
// 	"crypto/sha256"
// 	"encoding/hex"
// 	"errors"
// 	"fmt"
// 	"net/mail"
// 	"strings"
// 	"time"
//
// 	"github.com/example/goapi/internal/email"
// 	"github.com/example/goapi/internal/models"
// 	"github.com/example/goapi/internal/repository"
// 	"github.com/example/goapi/internal/utils"
// )
//
// // ErrInvalidCredentials is returned for any login failure. It is
// // intentionally generic (never "email not found" vs "wrong password") to
// // avoid leaking which emails are registered. The not-found path also runs a
// // dummy bcrypt comparison so the two paths take equal time.
// var ErrInvalidCredentials = errors.New("invalid email or password")
//
// // ErrEmailTaken is returned when registering with an email already in use.
// var ErrEmailTaken = errors.New("email is already registered")
//
// // ErrValidation wraps all input-validation failures.
// // var ErrValidation = errors.New("validation error")
//
// // AuthService implements registration, login and token refresh.
// type AuthService struct {
// 	users      *repository.UserRepository
// 	tokens     *repository.RefreshTokenRepository
// 	jwt        *utils.JWTManager
// 	email      *email.Service
// 	bcryptCost int
//
// 	outbox *repository.OutboxRepository
// }
//
// // NewAuthService constructs an AuthService.
// func NewAuthService(
// 	users *repository.UserRepository,
// 	tokens *repository.RefreshTokenRepository,
// 	jwt *utils.JWTManager,
// 	emailService *email.Service,
// 	bcryptCost int,
// 	outbox *repository.OutboxRepository,
// ) *AuthService {
// 	return &AuthService{
// 		users:      users,
// 		tokens:     tokens,
// 		jwt:        jwt,
// 		email:      emailService,
// 		bcryptCost: bcryptCost,
// 		outbox:     outbox,
// 	}
// }
//
// // Register creates a new user account and returns freshly issued tokens.
//
// func (s *AuthService) Register(
// 	ctx context.Context,
// 	req models.RegisterRequest,
// ) (*models.AuthResponse, error) {
// 	email, err := normalizeEmail(req.Email)
// 	if err != nil {
// 		return nil, fmt.Errorf(
// 			"%w: a valid email is required",
// 			ErrValidation,
// 		)
// 	}
//
// 	if err := validateRegisterRequest(req.Password, req.Name); err != nil {
// 		return nil, err
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
// 		// Role:         models.RoleUser,
// 		//
// 	}
//
// 	created, err := s.users.CreateWithOutbox(
// 		ctx,
// 		user,
// 		s.outbox,
// 	)
// 	if err != nil {
// 		if errors.Is(err, repository.ErrDuplicate) {
// 			return nil, ErrEmailTaken
// 		}
//
// 		return nil, fmt.Errorf("register user: %w", err)
// 	}
//
// 	s.email.SendWelcomeEmail(
// 		created.Email,
// 		created.Name,
// 	)
//
// 	return s.issueTokens(ctx, created)
// }
//
// // Login verifies credentials and issues new tokens on success.
// func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
// 	email, err := normalizeEmail(req.Email)
// 	if err != nil {
// 		// Malformed email still goes through the timing-equalizing path.
// 		_ = utils.CheckPassword(utils.DummyHash, req.Password)
// 		return nil, ErrInvalidCredentials
// 	}
//
// 	user, err := s.users.GetByEmail(ctx, email)
// 	if err != nil {
// 		if errors.Is(err, repository.ErrNotFound) {
// 			_ = utils.CheckPassword(utils.DummyHash, req.Password)
// 			return nil, ErrInvalidCredentials
// 		}
// 		return nil, fmt.Errorf("lookup user: %w", err)
// 	}
//
// 	if !utils.CheckPassword(user.PasswordHash, req.Password) {
// 		return nil, ErrInvalidCredentials
// 	}
//
// 	return s.issueTokens(ctx, user)
// }
//
// // Refresh validates a refresh token, revokes it, and issues a rotated pair.
// // Reusing an already-rotated (or revoked) token is rejected — if that
// // happens, the token may have been stolen, so all of the user's tokens are
// // revoked as a defensive measure.
// func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
// 	claims, err := s.jwt.Verify(refreshToken, utils.RefreshToken)
// 	if err != nil {
// 		return nil, utils.ErrInvalidToken
// 	}
//
// 	presentedHash := hashToken(refreshToken)
//
// 	rec, err := s.tokens.FindValid(ctx, presentedHash)
// 	if err != nil {
// 		if errors.Is(err, repository.ErrNotFound) {
// 			// Unknown/revoked/expired token presented but with a VALID JWT
// 			// signature: it was valid once. Possible theft — nuke the session.
// 			s.tokens.RevokeAllForUser(ctx, claims.UserID)
// 			return nil, utils.ErrInvalidToken
// 		}
// 		return nil, fmt.Errorf("lookup refresh token: %w", err)
// 	}
//
// 	// Rotate: the old token dies here.
// 	if err := s.tokens.Revoke(ctx, presentedHash); err != nil {
// 		return nil, fmt.Errorf("revoke refresh token: %w", err)
// 	}
//
// 	user, err := s.users.GetByID(ctx, rec.UserID)
// 	if err != nil {
// 		if errors.Is(err, repository.ErrNotFound) {
// 			return nil, utils.ErrInvalidToken
// 		}
// 		return nil, fmt.Errorf("lookup user: %w", err)
// 	}
//
// 	return s.issueTokens(ctx, user)
// }
//
// // issueTokens generates a fresh access+refresh pair and persists the refresh
// // token hash so Refresh can rotate and revoke it.
// func (s *AuthService) issueTokens(ctx context.Context, user *models.User) (*models.AuthResponse, error) {
// 	access, accessExpiresAt, err := s.jwt.GenerateAccessToken(user.ID, string(user.Role))
// 	if err != nil {
// 		return nil, fmt.Errorf("generate access token: %w", err)
// 	}
// 	refresh, refreshExpiresAt, err := s.jwt.GenerateRefreshToken(user.ID, string(user.Role))
// 	if err != nil {
// 		return nil, fmt.Errorf("generate refresh token: %w", err)
// 	}
//
// 	if err := s.tokens.Create(ctx, user.ID, hashToken(refresh), refreshExpiresAt); err != nil {
// 		return nil, fmt.Errorf("store refresh token: %w", err)
// 	}
//
// 	return &models.AuthResponse{
// 		AccessToken:  access,
// 		RefreshToken: refresh,
// 		TokenType:    "Bearer",
// 		ExpiresIn:    int64(time.Until(accessExpiresAt).Seconds()),
// 		User:         *user,
// 	}, nil
// }
//
// // hashToken returns the SHA-256 hex digest of a token. Only digests are
// // stored, so a database leak does not leak usable refresh tokens.
// func hashToken(token string) string {
// 	sum := sha256.Sum256([]byte(token))
// 	return hex.EncodeToString(sum[:])
// }
//
// func normalizeEmail(raw string) (string, error) {
// 	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
// 	if err != nil {
// 		return "", err
// 	}
// 	return strings.ToLower(addr.Address), nil
// }
//
// func validateRegisterRequest(password, name string) error {
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

// ErrInvalidCredentials is returned for any login failure.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrEmailTaken is returned when registering with an email already in use.
var ErrEmailTaken = errors.New("email is already registered")

// ErrValidation wraps input-validation failures.
// var ErrValidation = errors.New("validation error")

// AuthService implements registration, login and token refresh.
type AuthService struct {
	users      *repository.UserRepository
	tokens     *repository.RefreshTokenRepository
	jwt        *utils.JWTManager
	email      *email.Service
	bcryptCost int
	outbox     *repository.OutboxRepository
}

// NewAuthService constructs an AuthService.
func NewAuthService(
	users *repository.UserRepository,
	tokens *repository.RefreshTokenRepository,
	jwt *utils.JWTManager,
	emailService *email.Service,
	bcryptCost int,
	outbox *repository.OutboxRepository,
) *AuthService {
	return &AuthService{
		users:      users,
		tokens:     tokens,
		jwt:        jwt,
		email:      emailService,
		bcryptCost: bcryptCost,
		outbox:     outbox,
	}
}

// Register creates a new user account and returns freshly issued tokens.
func (s *AuthService) Register(
	ctx context.Context,
	req models.RegisterRequest,
) (*models.AuthResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: a valid email is required",
			ErrValidation,
		)
	}

	if err := validateRegisterRequest(req.Password, req.Name); err != nil {
		return nil, err
	}

	hash, err := utils.HashPassword(req.Password, s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	// RoleID is intentionally not set here.
	// UserRepository.CreateWithOutbox assigns the default "user" role
	// inside the same database transaction.
	user := &models.User{
		Email:        email,
		PasswordHash: hash,
		Name:         strings.TrimSpace(req.Name),
	}

	created, err := s.users.CreateWithOutbox(
		ctx,
		user,
		s.outbox,
	)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, ErrEmailTaken
		}

		return nil, fmt.Errorf("register user: %w", err)
	}

	s.email.SendWelcomeEmail(
		created.Email,
		created.Name,
	)

	return s.issueTokens(ctx, created)
}

// Login verifies credentials and issues new tokens on success.
func (s *AuthService) Login(
	ctx context.Context,
	req models.LoginRequest,
) (*models.AuthResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		// Keep malformed-email login timing similar to a real password check.
		_ = utils.CheckPassword(utils.DummyHash, req.Password)
		return nil, ErrInvalidCredentials
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Prevent account enumeration through timing differences.
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
func (s *AuthService) Refresh(
	ctx context.Context,
	refreshToken string,
) (*models.AuthResponse, error) {
	claims, err := s.jwt.Verify(
		refreshToken,
		utils.RefreshToken,
	)
	if err != nil {
		return nil, utils.ErrInvalidToken
	}

	presentedHash := hashToken(refreshToken)

	rec, err := s.tokens.FindValid(
		ctx,
		presentedHash,
	)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// A correctly signed but already-revoked token may indicate
			// refresh-token reuse. Revoke the user's remaining sessions.
			_ = s.tokens.RevokeAllForUser(
				ctx,
				claims.UserID,
			)

			return nil, utils.ErrInvalidToken
		}

		return nil, fmt.Errorf(
			"lookup refresh token: %w",
			err,
		)
	}

	// Rotate the refresh token.
	if err := s.tokens.Revoke(
		ctx,
		presentedHash,
	); err != nil {
		return nil, fmt.Errorf(
			"revoke refresh token: %w",
			err,
		)
	}

	user, err := s.users.GetByID(
		ctx,
		rec.UserID,
	)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, utils.ErrInvalidToken
		}

		return nil, fmt.Errorf(
			"lookup user: %w",
			err,
		)
	}

	return s.issueTokens(ctx, user)
}

// issueTokens generates a fresh access + refresh pair.
//
// RBAC uses RoleID rather than the old string Role field.
// The role's actual permissions remain in PostgreSQL.
func (s *AuthService) issueTokens(
	ctx context.Context,
	user *models.User,
) (*models.AuthResponse, error) {
	access, accessExpiresAt, err := s.jwt.GenerateAccessToken(
		user.ID,
		user.RoleID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"generate access token: %w",
			err,
		)
	}

	refresh, refreshExpiresAt, err := s.jwt.GenerateRefreshToken(
		user.ID,
		user.RoleID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"generate refresh token: %w",
			err,
		)
	}

	if err := s.tokens.Create(
		ctx,
		user.ID,
		hashToken(refresh),
		refreshExpiresAt,
	); err != nil {
		return nil, fmt.Errorf(
			"store refresh token: %w",
			err,
		)
	}

	return &models.AuthResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(accessExpiresAt).Seconds()),
		User:         *user,
	}, nil
}

// hashToken returns the SHA-256 hex digest of a token.
// Only the digest is stored in PostgreSQL.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(
		strings.TrimSpace(raw),
	)
	if err != nil {
		return "", err
	}

	return strings.ToLower(addr.Address), nil
}

func validateRegisterRequest(
	password string,
	name string,
) error {
	if len(password) < 8 {
		return fmt.Errorf(
			"%w: password must be at least 8 characters",
			ErrValidation,
		)
	}

	if len(password) > 72 {
		return fmt.Errorf(
			"%w: password must be at most 72 characters",
			ErrValidation,
		)
	}

	if strings.TrimSpace(name) == "" {
		return fmt.Errorf(
			"%w: name is required",
			ErrValidation,
		)
	}

	return nil
}
