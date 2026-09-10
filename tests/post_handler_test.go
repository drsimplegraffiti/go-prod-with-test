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
	"github.com/example/goapi/internal/middleware"
	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
	"github.com/example/goapi/internal/service"
	"github.com/example/goapi/internal/utils"
)

func newPostHandler(t *testing.T) (*handlers.PostHandler, sqlmock.Sqlmock, *utils.JWTManager) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	posts := repository.NewPostRepository(db)
	jwtManager := utils.NewJWTManager("test-secret-at-least-32-characters-long", time.Minute, time.Hour)
	return handlers.NewPostHandler(service.NewPostService(posts)), mock, jwtManager
}

// withAuth wraps a request with a valid bearer token, exercising the real
// Auth middleware end-to-end rather than injecting context directly.
func withAuth(req *http.Request, jwtManager *utils.JWTManager, userID string) *http.Request {
	token, _, _ := jwtManager.GenerateAccessToken(userID, "user")
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestCreatePostHandler_RequiresAuth(t *testing.T) {
	h, _, jwtManager := newPostHandler(t)

	body, _ := json.Marshal(models.CreatePostRequest{Title: "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler := middleware.Auth(jwtManager)(http.HandlerFunc(h.Create))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without Authorization header, got %d", rec.Code)
	}
}

func TestCreatePostHandler_Success(t *testing.T) {
	h, mock, jwtManager := newPostHandler(t)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO posts`)).
		WithArgs("user-1", "Hello World", "content here", models.StatusDraft).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}).
			AddRow("post-1", "user-1", "Hello World", "content here", "draft", now, now))

	body, _ := json.Marshal(models.CreatePostRequest{Title: "Hello World", Content: "content here"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", bytes.NewReader(body))
	req = withAuth(req, jwtManager, "user-1")
	rec := httptest.NewRecorder()

	handler := middleware.Auth(jwtManager)(http.HandlerFunc(h.Create))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListPostsHandler_AppliesQueryParams(t *testing.T) {
	h, mock, _ := newPostHandler(t)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM posts WHERE status = $1`)).
		WithArgs(models.StatusPublished).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`FROM posts`)).
		WithArgs(models.StatusPublished, 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}).
			AddRow("post-1", "user-1", "A", "a", "published", now, now).
			AddRow("post-2", "user-1", "B", "b", "published", now, now))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts?status=published&page=1&page_size=10&sort_by=title&sort_dir=asc", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp models.PaginatedResponse[models.Post]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.TotalItems != 2 || len(resp.Data) != 2 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestListPostsHandler_RejectsInvalidStatus(t *testing.T) {
	h, _, _ := newPostHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts?status=not-a-status", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdatePostHandler_ForbiddenForNonOwner(t *testing.T) {
	h, mock, jwtManager := newPostHandler(t)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, title, content, status, created_at, updated_at`)).
		WithArgs("post-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}).
			AddRow("post-1", "owner-1", "Title", "body", "draft", now, now))

	newTitle := "Hacked"
	body, _ := json.Marshal(models.UpdatePostRequest{Title: &newTitle})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/posts/post-1", bytes.NewReader(body))
	req.SetPathValue("id", "post-1")
	req = withAuth(req, jwtManager, "someone-else")
	rec := httptest.NewRecorder()

	handler := middleware.Auth(jwtManager)(http.HandlerFunc(h.Update))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeletePostHandler_NotFound(t *testing.T) {
	h, mock, jwtManager := newPostHandler(t)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, title, content, status, created_at, updated_at`)).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/missing", nil)
	req.SetPathValue("id", "missing")
	req = withAuth(req, jwtManager, "user-1")
	rec := httptest.NewRecorder()

	handler := middleware.Auth(jwtManager)(http.HandlerFunc(h.Delete))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
