package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/example/goapi/internal/models"
)

func newMockDB(t *testing.T) (*PostRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	return NewPostRepository(db), mock, func() { db.Close() }
}

func TestPostRepository_Create(t *testing.T) {
	repo, mock, closeFn := newMockDB(t)
	defer closeFn()

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}).
		AddRow("post-1", "user-1", "Hello", "World", "draft", now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO posts`)).
		WithArgs("user-1", "Hello", "World", models.StatusDraft).
		WillReturnRows(rows)

	post, err := repo.Create(context.Background(), &models.Post{
		UserID: "user-1", Title: "Hello", Content: "World", Status: models.StatusDraft,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if post.ID != "post-1" {
		t.Errorf("expected ID post-1, got %s", post.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPostRepository_GetByID_NotFound(t *testing.T) {
	repo, mock, closeFn := newMockDB(t)
	defer closeFn()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, title, content, status, created_at, updated_at`)).
		WithArgs("missing-id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}))

	_, err := repo.GetByID(context.Background(), "missing-id")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPostRepository_List_AppliesFiltersAndCountsSeparately(t *testing.T) {
	repo, mock, closeFn := newMockDB(t)
	defer closeFn()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM posts WHERE status = $1 AND title ILIKE $2`)).
		WithArgs(models.StatusPublished, "%golang%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`FROM posts`)).
		WithArgs(models.StatusPublished, "%golang%", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "content", "status", "created_at", "updated_at"}).
			AddRow("post-1", "user-1", "Learning Golang", "body", "published", now, now))

	posts, total, err := repo.List(context.Background(), models.PostListParams{
		Page: 1, PageSize: 20, Status: models.StatusPublished, Search: "golang",
		SortBy: "created_at", SortDir: "desc",
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(posts) != 1 || posts[0].Title != "Learning Golang" {
		t.Errorf("unexpected posts result: %+v", posts)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPostRepository_Delete_NotFound(t *testing.T) {
	repo, mock, closeFn := newMockDB(t)
	defer closeFn()

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM posts WHERE id = $1 AND user_id = $2`)).
		WithArgs("post-1", "someone-else").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Delete(context.Background(), "post-1", "someone-else")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
