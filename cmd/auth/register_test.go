package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bifu-cli/internal/clifconfig"
)

func TestDoRegisterAndActivate(t *testing.T) {
	var gotReg map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/user/register":
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &gotReg)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"retCode": "0",
				"result":  map[string]any{"issueId": "iss-1", "email": "x@y.z"},
			})
		case "/user/activate":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"retCode": "0",
				"result":  map[string]any{"cookieStr": `{"Name":"929a528c9d53c705","Value":"REGcookie=="}`},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	issueID, err := doRegister(srv.URL, "x@y.z", "Pw123!@#", "en", "ref9", "API")
	if err != nil {
		t.Fatalf("doRegister: %v", err)
	}
	if issueID != "iss-1" {
		t.Errorf("issueID = %q, want iss-1", issueID)
	}
	// confirmPassword must mirror password, and referrer passed through.
	if gotReg["confirmPassword"] != "Pw123!@#" || gotReg["referrer"] != "ref9" {
		t.Errorf("register body = %v", gotReg)
	}

	name, val, err := doActivate(srv.URL, issueID, "123456", "API")
	if err != nil {
		t.Fatalf("doActivate: %v", err)
	}
	if name != "929a528c9d53c705" || val != "REGcookie==" {
		t.Errorf("activate cookie = (%q,%q)", name, val)
	}
}

func TestDoRegisterError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"retCode":"212","retMsg":"restricted region"}`))
	}))
	defer srv.Close()
	if _, err := doRegister(srv.URL, "x@y.z", "p", "en", "", "API"); err == nil ||
		!strings.Contains(err.Error(), "212") {
		t.Errorf("expected 212 error, got %v", err)
	}
}

func TestDoLogoutSendsCookie(t *testing.T) {
	var gotCookie, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"retCode":"0"}`))
	}))
	defer srv.Close()

	p := &clifconfig.Profile{BaseURL: srv.URL}
	p.Auth.AuthCookie = "abc=="
	p.Auth.AuthCookieName = "929a528c9d53c705"
	if err := doLogout(p); err != nil {
		t.Fatalf("doLogout: %v", err)
	}
	if gotPath != "/user/logout" {
		t.Errorf("path = %q", gotPath)
	}
	if gotCookie != "929a528c9d53c705=abc==" {
		t.Errorf("cookie sent = %q, want env-specific name", gotCookie)
	}
}
