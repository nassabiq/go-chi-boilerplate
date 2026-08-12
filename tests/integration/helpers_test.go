//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
)

// errorBody mirrors internal/utils/response.Error.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// loginResult captures everything a test needs after a successful login.
type loginResult struct {
	Auth   dto.Auth
	UserID int64
	SID    string
}

// do sends an HTTP request through the real router and returns the response.
func (e *testEnv) do(method, path string, body any, headers map[string]string) *http.Response {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		reader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec.Result()
}

// doRaw sends a request with an already-encoded body, so callers can
// submit malformed JSON (which json.Marshal cannot produce).
func (e *testEnv) doRaw(method, path string, body []byte, headers map[string]string) *http.Response {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec.Result()
}

// authed sends an authenticated request using a Bearer token.
func (e *testEnv) authed(method, path, token string, body any) *http.Response {
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return e.do(method, path, body, headers)
}

func decode(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func decodeError(t *testing.T, resp *http.Response) errorBody {
	t.Helper()
	defer resp.Body.Close()
	var eb errorBody
	if err := json.NewDecoder(resp.Body).Decode(&eb); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return eb
}

func requireStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected status %d, got %d (body: %s)", want, resp.StatusCode, string(body))
	}
}

func requireCode(t *testing.T, resp *http.Response, want string) {
	t.Helper()
	eb := decodeError(t, resp)
	if eb.Code != want {
		t.Fatalf("expected error code %s, got %s (%s)", want, eb.Code, eb.Message)
	}
}

// login performs POST /auth/login and returns the parsed result. If ua is
// non-empty it is sent as the User-Agent header (transport metadata).
func (e *testEnv) login(t *testing.T, email, password, ua string) loginResult {
	t.Helper()

	headers := map[string]string{}
	if ua != "" {
		headers["User-Agent"] = ua
	}

	resp := e.do(http.MethodPost, "/auth/login", payloads.LoginRequest{
		Email:    email,
		Password: password,
	}, headers)
	requireStatus(t, resp, http.StatusOK)

	var auth dto.Auth
	decode(t, resp, &auth)

	claims, err := e.jwt.ParseAccessToken(auth.AccessToken)
	if err != nil {
		t.Fatalf("parse access token from login: %v", err)
	}

	return loginResult{Auth: auth, UserID: claims.UserID, SID: claims.SessionID}
}

// loginRaw performs POST /auth/login and returns the raw response (no asserts).
func (e *testEnv) loginRaw(t *testing.T, email, password, ua string) *http.Response {
	t.Helper()
	headers := map[string]string{}
	if ua != "" {
		headers["User-Agent"] = ua
	}
	return e.do(http.MethodPost, "/auth/login", payloads.LoginRequest{
		Email:    email,
		Password: password,
	}, headers)
}

// refresh performs POST /auth/refresh and returns the parsed result.
func (e *testEnv) refresh(t *testing.T, refreshToken string) (dto.Auth, *http.Response) {
	t.Helper()
	resp := e.do(http.MethodPost, "/auth/refresh", payloads.RefreshTokenRequest{
		RefreshToken: refreshToken,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		return dto.Auth{}, resp
	}
	var auth dto.Auth
	decode(t, resp, &auth)
	return auth, resp
}

// createUserFixture inserts a real user using the application's bcrypt hashing,
// so login proceeds through the genuine password path.
func (e *testEnv) createUserFixture(t *testing.T, email, password string, status enums.Status) int64 {
	t.Helper()

	hash, err := auth.HashPassword(password, 12)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := &entities.User{
		Name:     strings.Split(email, "@")[0],
		Email:    email,
		Password: hash,
		StatusID: status,
		Version:  1,
	}

	if err := e.db.WithContext(context.Background()).Create(user).Error; err != nil {
		t.Fatalf("create user fixture: %v", err)
	}
	return user.ID
}

// hGetAll returns the raw Redis hash for a session key.
func (e *testEnv) hGetAll(ctx context.Context, key string) map[string]string {
	res, err := e.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return map[string]string{}
	}
	return res
}

func (e *testEnv) sIsMember(ctx context.Context, key, member string) bool {
	ok, err := e.redis.SIsMember(ctx, key, member).Result()
	if err != nil {
		return false
	}
	return ok
}

func (e *testEnv) exists(ctx context.Context, key string) bool {
	n, err := e.redis.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return n > 0
}

// sessionKey builds the canonical session key used by SessionManager.
func sessionKey(sid string) string {
	return "auth:session:" + sid
}

func userSessionsKey(userID int64) string {
	return fmt.Sprintf("auth:user:sessions:%d", userID)
}
