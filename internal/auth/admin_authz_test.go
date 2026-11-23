package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"bootimus/internal/models"

	"gorm.io/gorm"
)

type fakeUserStore struct {
	users      map[string]*models.User
	getUserErr error
}

func (f *fakeUserStore) EnsureAdminUser() (string, string, bool, error) {
	return "admin", "", false, nil
}
func (f *fakeUserStore) ResetAdminPassword() (string, error) { return "", nil }
func (f *fakeUserStore) UpdateUserLastLogin(string) error    { return nil }
func (f *fakeUserStore) GetUser(username string) (*models.User, error) {
	if f.getUserErr != nil {
		return nil, f.getUserErr
	}
	u, ok := f.users[username]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	c := *u
	return &c, nil
}

func TestAuthenticationDistinguishesStoreFailureFromRevocation(t *testing.T) {
	store := &fakeUserStore{users: map[string]*models.User{
		"admin": {Username: "admin", Enabled: true, IsAdmin: true},
	}}
	m := &Manager{userStore: store, jwtSecret: []byte("test-secret-0123456789")}
	token, err := m.GenerateToken("admin", true)
	if err != nil {
		t.Fatal(err)
	}
	handler := m.JWTMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	call := func() int {
		req := httptest.NewRequest(http.MethodGet, "/api/images", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec.Code
	}
	store.getUserErr = errors.New("database temporarily unavailable")
	if got := call(); got != http.StatusServiceUnavailable {
		t.Fatalf("store failure: want 503, got %d", got)
	}
	store.getUserErr = nil
	if got := call(); got != http.StatusOK {
		t.Fatalf("same session after recovery: want 200, got %d", got)
	}
	store.users["admin"].Enabled = false
	if got := call(); got != http.StatusUnauthorized {
		t.Fatalf("disabled account: want 401, got %d", got)
	}
	delete(store.users, "admin")
	if got := call(); got != http.StatusUnauthorized {
		t.Fatalf("deleted account: want 401, got %d", got)
	}
}
func (f *fakeUserStore) CreateUser(user *models.User) error {
	c := *user
	f.users[user.Username] = &c
	return nil
}
func (f *fakeUserStore) UpdateUser(username string, user *models.User) error {
	c := *user
	f.users[username] = &c
	return nil
}

func TestAdminMiddlewareRequiresAdmin(t *testing.T) {
	store := &fakeUserStore{users: map[string]*models.User{
		"alice": {Username: "alice", Enabled: true, IsAdmin: true},
		"bob":   {Username: "bob", Enabled: true, IsAdmin: false},
	}}
	m := &Manager{userStore: store, jwtSecret: []byte("test-secret-0123456789")}

	tok := func(user string, isAdmin bool) string {
		s, err := m.GenerateToken(user, isAdmin)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	call := func(authHeader string) int {
		h := m.AdminMiddleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec.Code
	}

	if got := call(""); got != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", got)
	}
	if got := call("Bearer " + tok("alice", true)); got != http.StatusOK {
		t.Fatalf("admin: want 200, got %d", got)
	}
	if got := call("Bearer " + tok("bob", false)); got != http.StatusForbidden {
		t.Fatalf("non-admin: want 403, got %d", got)
	}
	if got := call("Bearer " + tok("bob", true)); got != http.StatusForbidden {
		t.Fatalf("forged admin claim: want 403, got %d", got)
	}
	store.users["alice"].IsAdmin = false
	if got := call("Bearer " + tok("alice", true)); got != http.StatusForbidden {
		t.Fatalf("demoted admin: want 403, got %d", got)
	}
}
