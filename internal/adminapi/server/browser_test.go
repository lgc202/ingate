package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBrowserSessionOrigin(t *testing.T) {
	for _, tt := range []struct {
		name   string
		origin string
		header string
		cookie string
		fetch  string
		bearer string
		status int
	}{
		{name: "same origin", origin: "http://127.0.0.1:5173", header: "1", cookie: "ingate_session=opaque", status: 204},
		{name: "cross origin", origin: "https://attacker.example", header: "1", cookie: "ingate_session=opaque", status: 403},
		{name: "no origin", header: "1", status: 403},
		{name: "no custom header", origin: "http://127.0.0.1:5173", cookie: "ingate_session=opaque", status: 403},
		{name: "duplicate cookie", origin: "http://127.0.0.1:5173", header: "1", cookie: "ingate_session=a; ingate_session=b", status: 403},
		{name: "cross site metadata", origin: "http://127.0.0.1:5173", header: "1", cookie: "ingate_session=opaque", fetch: "cross-site", status: 403},
		{name: "mixed credentials", origin: "http://127.0.0.1:5173", header: "1", cookie: "ingate_session=opaque", bearer: "Bearer other", status: 403},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:5173/api/admin/v1/auth/login", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set(browserHeader, tt.header)
			req.Header.Set("Cookie", tt.cookie)
			req.Header.Set("Sec-Fetch-Site", tt.fetch)
			req.Header.Set("Authorization", tt.bearer)
			called := false
			next := browserSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Header.Get("Authorization") != "Bearer opaque" {
					t.Errorf("forwarded Authorization = %q, want opaque cookie credential", r.Header.Get("Authorization"))
				}

				w.WriteHeader(http.StatusNoContent)
			}))

			response := httptest.NewRecorder()
			next.ServeHTTP(response, req)

			if response.Code != tt.status || called != (tt.status == 204) {
				t.Errorf("browserSession(%s) status/called = %d/%t, want %d/%t", tt.name, response.Code, called, tt.status, tt.status == 204)
			}
		})
	}
}

func TestBrowserReplyHidesToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:5173/api/admin/v1/auth/login", nil)
	req.Header.Set(browserHeader, "1")
	reply := &adminv1.LoginReply{SessionToken: "opaque", ExpireTime: timestamppb.New(time.Now().Add(time.Hour))}
	response := httptest.NewRecorder()

	public := browserReply(response, req, reply).(*adminv1.LoginReply)

	if public.SessionToken != "" || reply.SessionToken != "opaque" {
		t.Errorf("browserReply token = %q, original = %q, want empty/unchanged", public.SessionToken, reply.SessionToken)
	}

	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != "opaque" || !cookies[0].HttpOnly || !cookies[0].Secure ||
		cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/api/admin/v1" {
		t.Fatalf("browserReply cookies = %v, want secure HttpOnly SameSiteStrict session cookie", cookies)
	}

	logout := httptest.NewRecorder()
	browserReply(logout, req, &adminv1.LogoutReply{})
	if logout.Result().Cookies()[0].MaxAge != -1 {
		t.Error("browserReply(logout) did not expire cookie")
	}

	req.Header.Del(browserHeader)
	if browserReply(httptest.NewRecorder(), req, reply) != reply {
		t.Error("browserReply(Bearer client) changed login reply")
	}
}

func TestBrowserOriginSchemes(t *testing.T) {
	for _, tt := range []struct {
		origin string
		want   bool
	}{
		{origin: "https://console.example", want: true},
		{origin: "http://console.example", want: false},
		{origin: "null", want: false},
		{origin: "https://console.example/", want: false},
		{origin: "https://console.example.attacker.example", want: false},
	} {
		req := httptest.NewRequest(http.MethodPost, "https://console.example/api/admin/v1/auth/login", nil)
		req.Header.Set("Origin", tt.origin)

		if got := sameBrowserOrigin(req); got != tt.want {
			t.Errorf("sameBrowserOrigin(%q) = %t, want %t", tt.origin, got, tt.want)
		}
	}
}

func TestBrowserSessionPreservesBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8000/api/admin/v1/accounts/me", nil)
	req.Header.Set("Authorization", "Bearer cli-token")
	response := httptest.NewRecorder()
	next := browserSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cli-token" {
			t.Error("browserSession(Bearer) changed credential")
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	next.ServeHTTP(response, req)

	if response.Code != http.StatusNoContent {
		t.Errorf("browserSession(Bearer) status = %d, want 204", response.Code)
	}
}
