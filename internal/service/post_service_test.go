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
)

func newPostTestService(t *testing.T) (*PostService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	posts := repository.NewPostRepository(db)
	return NewPostService(posts), mock, func() { db.Close() }
}

func TestPostService_Create_Success(t *testing.T) {
	svc, mock, closeFn := newPostTestService(t)
	defer closeFn()

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO posts`)).
		WithArgs("user-1", "My Title", "body", models.StatusDraft).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}).
			AddRow("post-1", "user-1", "My Title", "body", "draft", now, now))

	post, err := svc.Create(context.Background(), "user-1", models.CreatePostRequest{
		Title: "  My Title  ", Content: "body",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if post.Title != "My Title" {
		t.Errorf("expected trimmed title, got %q", post.Title)
	}
}

func TestPostService_Create_RejectsEmptyTitle(t *testing.T) {
	svc, _, closeFn := newPostTestService(t)
	defer closeFn()

	_, err := svc.Create(context.Background(), "user-1", models.CreatePostRequest{Title: "   "})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for empty title, got %v", err)
	}
}

func TestPostService_Create_RejectsInvalidStatus(t *testing.T) {
	svc, _, closeFn := newPostTestService(t)
	defer closeFn()

	_, err := svc.Create(context.Background(), "user-1", models.CreatePostRequest{
		Title: "Valid", Status: "not-a-real-status",
	})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for invalid status, got %v", err)
	}
}

func TestPostService_Update_ForbidsNonOwner(t *testing.T) {
	svc, mock, closeFn := newPostTestService(t)
	defer closeFn()

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, title, content, status, created_at, updated_at`)).
		WithArgs("post-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}).
			AddRow("post-1", "owner-1", "Title", "body", "draft", now, now))

	newTitle := "New Title"
	_, err := svc.Update(context.Background(), "post-1", "someone-else", models.UpdatePostRequest{Title: &newTitle})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestPostService_Update_NotFound(t *testing.T) {
	svc, mock, closeFn := newPostTestService(t)
	defer closeFn()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, title, content, status, created_at, updated_at`)).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}))

	newTitle := "New Title"
	_, err := svc.Update(context.Background(), "missing", "user-1", models.UpdatePostRequest{Title: &newTitle})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPostService_Delete_ForbidsNonOwner(t *testing.T) {
	svc, mock, closeFn := newPostTestService(t)
	defer closeFn()

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, title, content, status, created_at, updated_at`)).
		WithArgs("post-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}).
			AddRow("post-1", "owner-1", "Title", "body", "draft", now, now))

	err := svc.Delete(context.Background(), "post-1", "someone-else")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestPostService_List_RejectsInvalidStatusFilter(t *testing.T) {
	svc, _, closeFn := newPostTestService(t)
	defer closeFn()

	_, err := svc.List(context.Background(), models.PostListParams{
		Page: 1, PageSize: 20, Status: "bogus",
	})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestPostService_List_ComputesPaginationMetadata(t *testing.T) {
	svc, mock, closeFn := newPostTestService(t)
	defer closeFn()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM posts`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(45))

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"})
	for i := 0; i < 20; i++ {
		rows.AddRow("post", "user", "title", "body", "draft", now, now)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`FROM posts`)).WillReturnRows(rows)

	result, err := svc.List(context.Background(), models.PostListParams{
		Page: 1, PageSize: 20, SortBy: "created_at", SortDir: "desc",
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.TotalItems != 45 {
		t.Errorf("expected TotalItems 45, got %d", result.TotalItems)
	}
	if result.TotalPages != 3 {
		t.Errorf("expected TotalPages 3 (ceil(45/20)), got %d", result.TotalPages)
	}
	if len(result.Data) != 20 {
		t.Errorf("expected 20 items in page, got %d", len(result.Data))
	}
}
