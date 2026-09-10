package service

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
	"github.com/example/goapi/internal/utils"
)

const testCost = 4 // cheap bcrypt cost so tests run fast

func newAuthTestService(t *testing.T) (*AuthService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	users := repository.NewUserRepository(db)
	jwtManager := utils.NewJWTManager("test-secret-at-least-32-characters-long", time.Minute, time.Hour)
	svc := NewAuthService(users, jwtManager, testCost)
	return svc, mock, func() { db.Close() }
}

func TestAuthService_Register_Success(t *testing.T) {
	svc, mock, closeFn := newAuthTestService(t)
	defer closeFn()

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO users`)).
		WithArgs("new@example.com", sqlmock.AnyArg(), "New User", models.RoleUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "name", "role", "created_at", "updated_at"}).
			AddRow("user-1", "new@example.com", "hashed", "New User", "user", now, now))

	resp, err := svc.Register(context.Background(), models.RegisterRequest{
		Email: "New@Example.com", Password: "supersecret", Name: "New User",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Error("expected both tokens to be issued")
	}
	if resp.User.Email != "new@example.com" {
		t.Errorf("expected normalized email, got %s", resp.User.Email)
	}
}

func TestAuthService_Register_ValidationErrors(t *testing.T) {
	svc, _, closeFn := newAuthTestService(t)
	defer closeFn()

	cases := []models.RegisterRequest{
		{Email: "not-an-email", Password: "supersecret", Name: "X"},
		{Email: "a@b.com", Password: "short", Name: "X"},
		{Email: "a@b.com", Password: "supersecret", Name: "  "},
	}
	for _, req := range cases {
		if _, err := svc.Register(context.Background(), req); !errors.Is(err, ErrValidation) {
			t.Errorf("request %+v: expected ErrValidation, got %v", req, err)
		}
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	svc, mock, closeFn := newAuthTestService(t)
	defer closeFn()

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO users`)).
		WillReturnError(repository.ErrDuplicate)

	_, err := svc.Register(context.Background(), models.RegisterRequest{
		Email: "dup@example.com", Password: "supersecret", Name: "Dup",
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	svc, mock, closeFn := newAuthTestService(t)
	defer closeFn()

	hash, err := utils.HashPassword("correcthorse", testCost)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, email, password_hash, name, role, created_at, updated_at`)).
		WithArgs("user@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "name", "role", "created_at", "updated_at"}).
			AddRow("user-1", "user@example.com", hash, "User", "user", now, now))

	resp, err := svc.Login(context.Background(), models.LoginRequest{
		Email: "user@example.com", Password: "correcthorse",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if resp.User.ID != "user-1" {
		t.Errorf("expected user-1, got %s", resp.User.ID)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	svc, mock, closeFn := newAuthTestService(t)
	defer closeFn()

	hash, _ := utils.HashPassword("correcthorse", testCost)
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, email, password_hash, name, role, created_at, updated_at`)).
		WithArgs("user@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "name", "role", "created_at", "updated_at"}).
			AddRow("user-1", "user@example.com", hash, "User", "user", now, now))

	_, err := svc.Login(context.Background(), models.LoginRequest{
		Email: "user@example.com", Password: "wrong-password",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_Login_UnknownEmailGivesGenericError(t *testing.T) {
	svc, mock, closeFn := newAuthTestService(t)
	defer closeFn()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, email, password_hash, name, role, created_at, updated_at`)).
		WithArgs("nobody@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "name", "role", "created_at", "updated_at"}))

	_, err := svc.Login(context.Background(), models.LoginRequest{
		Email: "nobody@example.com", Password: "whatever",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials (not a distinct 'not found' error), got %v", err)
	}
}

func TestAuthService_Refresh_InvalidToken(t *testing.T) {
	svc, _, closeFn := newAuthTestService(t)
	defer closeFn()

	if _, err := svc.Refresh(context.Background(), "not-a-real-token"); !errors.Is(err, utils.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAuthService_Refresh_RejectsAccessTokenUsedAsRefresh(t *testing.T) {
	svc, _, closeFn := newAuthTestService(t)
	defer closeFn()

	jwtManager := utils.NewJWTManager("test-secret-at-least-32-characters-long", time.Minute, time.Hour)
	accessToken, _, _ := jwtManager.GenerateAccessToken("user-1", "user")

	if _, err := svc.Refresh(context.Background(), accessToken); !errors.Is(err, utils.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken when passing an access token to Refresh, got %v", err)
	}
}
