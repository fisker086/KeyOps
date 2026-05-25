package system

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupSettingTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &SettingHandler{}
	r := gin.New()
	r.POST("/ldap", h.TestLDAPConnection)
	r.POST("/sso", h.TestSSOConnection)
	return r
}

func TestTestLDAPConnection_InvalidRequest(t *testing.T) {
	r := setupSettingTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ldap", bytes.NewBufferString(`{"server":"ldap://127.0.0.1:389"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTestLDAPConnection_ConnectionFailed(t *testing.T) {
	r := setupSettingTestRouter()
	payload := map[string]interface{}{
		"server":       "127.0.0.1:1",
		"baseDN":       "dc=example,dc=com",
		"bindDN":       "cn=admin,dc=example,dc=com",
		"bindPassword": "secret",
		"timeout":      1,
	}
	b, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ldap", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTestSSOConnection_Success(t *testing.T) {
	r := setupSettingTestRouter()

	payload := map[string]interface{}{
		"provider":     "oidc",
		"clientId":     "cid",
		"clientSecret": "sec",
		"authUrl":      "https://example.com/auth",
		"tokenUrl":     "https://example.com/token",
		"userInfoUrl":  "https://example.com/userinfo",
	}
	b, _ := json.Marshal(payload)

	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("ok")),
			Header:     make(http.Header),
		}, nil
	})
	defer func() { http.DefaultTransport = oldTransport }()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sso", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTestSSOConnection_TokenEndpointFailed(t *testing.T) {
	r := setupSettingTestRouter()

	payload := map[string]interface{}{
		"provider":     "oidc",
		"clientId":     "cid",
		"clientSecret": "sec",
		"authUrl":      "https://example.com/auth",
		"tokenUrl":     "https://example.com/token",
	}
	b, _ := json.Marshal(payload)

	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/token" {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString("error")),
				Header:     make(http.Header),
			}, nil
		}
		return nil, errors.New("network error")
	})
	defer func() { http.DefaultTransport = oldTransport }()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sso", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
