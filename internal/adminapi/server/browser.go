package server

import (
	"net"
	"net/http"
	"net/url"
	"time"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/errno"
	"google.golang.org/protobuf/proto"
)

const (
	browserHeader = "X-Ingate-Browser"
	sessionCookie = "ingate_session"
)

// browserSession 只适配 HTTP 凭据传输，身份校验仍由原有 RequireSession 完成。
// 自定义头与同源 Origin 校验在正文解码前执行，不为跨源请求开放 CORS。
func browserSession(next http.Handler) http.Handler {
	protection := http.NewCrossOriginProtection()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookies := r.CookiesNamed(sessionCookie)
		browser := r.Header.Get(browserHeader) == "1" || len(cookies) > 0
		if !browser {
			next.ServeHTTP(w, r)

			return
		}

		w.Header().Set("Cache-Control", "no-store")

		unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		if len(cookies) > 1 || r.Header.Get("Authorization") != "" ||
			protection.Check(r) != nil ||
			(unsafe && (r.Header.Get(browserHeader) != "1" || !sameBrowserOrigin(r))) {
			encodeHTTPError(w, r, errno.ErrBrowserRequestForbidden)

			return
		}

		forwarded := r.Clone(r.Context())
		if len(cookies) == 1 {
			forwarded.Header.Set("Authorization", "Bearer "+cookies[0].Value)
		}

		next.ServeHTTP(w, forwarded)
	})
}

// browserReply 将已有 Session 放入 HttpOnly Cookie，返回副本避免修改业务返回值。
// Secure 始终开启；本地开发依赖浏览器对 loopback 的安全上下文支持，部署入口必须使用 HTTPS。
func browserReply(w http.ResponseWriter, r *http.Request, value any) any {
	if r.Header.Get(browserHeader) != "1" {
		return value
	}

	w.Header().Set("Cache-Control", "no-store")
	switch reply := value.(type) {
	case *adminv1.LoginReply:
		setSessionCookie(w, reply.GetSessionToken(), reply.GetExpireTime().AsTime())
		public := proto.CloneOf(reply)
		public.SessionToken = ""

		return public
	case *adminv1.LogoutReply, *adminv1.ChangePasswordReply:
		setSessionCookie(w, "", time.Unix(1, 0))
	}

	return value
}

func setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/api/admin/v1",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
	}
	if token == "" {
		cookie.MaxAge = -1
	}

	http.SetCookie(w, cookie)
}

func sameBrowserOrigin(r *http.Request) bool {
	values := r.Header.Values("Origin")
	if len(values) != 1 {
		return false
	}

	origin, err := url.Parse(values[0])
	if err != nil || origin.User != nil || origin.Host != r.Host || origin.Path != "" ||
		origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}

	if origin.Scheme == "https" {
		return true
	}

	// 仅允许本地开发的 HTTP Origin；不信任 X-Forwarded-* 来扩大身份边界。
	host := origin.Hostname()
	ip := net.ParseIP(host)

	return origin.Scheme == "http" && (host == "localhost" || (ip != nil && ip.IsLoopback()))
}
