package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/example/goapi/internal/handlers"
	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
	"github.com/example/goapi/internal/service"
	"github.com/example/goapi/internal/utils"
)

const testBcryptCost = 4

func newAuthHandler(t *testing.T) (*handlers.AuthHandler, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	users := repository.NewUserRepository(db)
	jwtManager := utils.NewJWTManager("test-secret-at-least-32-characters-long", time.Minute, time.Hour)
	authService := service.NewAuthService(users, jwtManager, testBcryptCost)
	return handlers.NewAuthHandler(authService), mock
}

func TestRegisterHandler_Success(t *testing.T) {
	h, mock := newAuthHandler(t)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO users`)).
		WithArgs("jane@example.com", sqlmock.AnyArg(), "Jane Doe", models.RoleUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "name", "role", "created_at", "updated_at"}).
			AddRow("user-1", "jane@example.com", "hashed", "Jane Doe", "user", now, now))

	body, _ := json.Marshal(models.RegisterRequest{
		Email: "jane@example.com", Password: "supersecret", Name: "Jane Doe",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp models.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected an access token in the response")
	}
}

func TestRegisterHandler_InvalidBody(t *testing.T) {
	h, _ := newAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte(`{not-json`)))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRegisterHandler_ValidationError(t *testing.T) {
	h, _ := newAuthHandler(t)

	body, _ := json.Marshal(models.RegisterRequest{Email: "not-an-email", Password: "short", Name: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	h, mock := newAuthHandler(t)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, email, password_hash, name, role, created_at, updated_at`)).
		WithArgs("nobody@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "name", "role", "created_at", "updated_at"}))

	body, _ := json.Marshal(models.LoginRequest{Email: "nobody@example.com", Password: "whatever"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRefreshHandler_MissingToken(t *testing.T) {
	h, _ := newAuthHandler(t)

	body, _ := json.Marshal(models.RefreshRequest{RefreshToken: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Refresh(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
