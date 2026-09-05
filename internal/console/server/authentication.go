package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lgc202/ingate/internal/console/conf"
	"github.com/lgc202/ingate/internal/pkg/adminidentity"
)

const (
	sessionCookieName     = "ingate_session"
	maxLoginBodyBytes     = 16 << 10
	maxSessionCookieBytes = 512
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sessionResponse struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username"`
}

// SessionAuth 为单管理员控制台签发和校验无状态会话。
// 它只解决开源单实例的管理入口保护，不建立用户、角色或权限模型。
type SessionAuth struct {
	enabled      bool
	username     string
	passwordHash [sha256.Size]byte
	secret       []byte
	ttl          time.Duration
	secureCookie bool
}

// NewSessionAuth 根据 Console 配置创建管理面会话认证。
func NewSessionAuth(config *conf.Server) *SessionAuth {
	auth := config.GetAuthentication()
	return &SessionAuth{
		enabled:      auth.GetEnabled(),
		username:     auth.GetUsername(),
		passwordHash: sha256.Sum256([]byte(auth.GetPassword())),
		secret:       []byte(auth.GetSessionSecret()),
		ttl:          auth.GetSessionTtl().AsDuration(),
		secureCookie: auth.GetSecureCookie(),
	}
}

// HandleSession 提供登录状态查询、登录和退出三个同源接口。
func (a *SessionAuth) HandleSession(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	switch request.Method {
	case http.MethodGet:
		a.writeCurrentSession(response, request)
	case http.MethodPost:
		a.login(response, request)
	case http.MethodDelete:
		a.logout(response)
	default:
		response.Header().Set("Allow", "GET, POST, DELETE")
		writeResponse(response, http.StatusMethodNotAllowed, "请求方法不支持", nil)
	}
}

// Protect 验证管理 API 会话，并把可信管理员身份传给内部 Admin API。
func (a *SessionAuth) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		request.Header.Del(adminidentity.Header)
		username, ok := a.currentUser(request)
		if !ok {
			writeResponse(response, http.StatusUnauthorized, "登录状态已失效，请重新登录", nil)
			return
		}
		request.Header.Set(adminidentity.Header, username)
		next.ServeHTTP(response, request)
	})
}

func (a *SessionAuth) writeCurrentSession(response http.ResponseWriter, request *http.Request) {
	username, authenticated := a.currentUser(request)
	writeResponse(response, http.StatusOK, "", sessionResponse{
		Authenticated: authenticated,
		Username:      username,
	})
}

func (a *SessionAuth) login(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, maxLoginBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var credentials loginRequest
	if err := decoder.Decode(&credentials); err != nil {
		writeResponse(response, http.StatusBadRequest, "登录信息格式不正确", nil)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeResponse(response, http.StatusBadRequest, "登录信息格式不正确", nil)
		return
	}
	if !a.validCredentials(credentials) {
		writeResponse(response, http.StatusUnauthorized, "用户名或密码不正确", nil)
		return
	}

	expiresAt := time.Now().Add(a.ttl)
	http.SetCookie(response, &http.Cookie{
		Name:     sessionCookieName,
		Value:    a.signSession(expiresAt),
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(a.ttl / time.Second),
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	writeResponse(response, http.StatusOK, "", sessionResponse{Authenticated: true, Username: a.username})
}

func (a *SessionAuth) logout(response http.ResponseWriter) {
	http.SetCookie(response, &http.Cookie{
		Name:     sessionCookieName,
		Path:     "/",
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	writeResponse(response, http.StatusOK, "", sessionResponse{})
}

func (a *SessionAuth) currentUser(request *http.Request) (string, bool) {
	if !a.enabled {
		return a.username, true
	}
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil || len(cookie.Value) > maxSessionCookieBytes {
		return "", false
	}
	payloadText, signatureText, ok := strings.Cut(cookie.Value, ".")
	if !ok || strings.Contains(signatureText, ".") {
		return "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadText)
	if err != nil {
		return "", false
	}
	signature, err := base64.RawURLEncoding.DecodeString(signatureText)
	if err != nil || !hmac.Equal(signature, a.signature(payload)) {
		return "", false
	}
	username, expiresText, ok := strings.Cut(string(payload), "\n")
	if !ok || username != a.username {
		return "", false
	}
	expiresAt, err := strconv.ParseInt(expiresText, 10, 64)
	if err != nil || time.Now().Unix() >= expiresAt {
		return "", false
	}
	return username, true
}

func (a *SessionAuth) validCredentials(credentials loginRequest) bool {
	usernameMatch := subtle.ConstantTimeCompare([]byte(credentials.Username), []byte(a.username))
	passwordHash := sha256.Sum256([]byte(credentials.Password))
	passwordMatch := subtle.ConstantTimeCompare(passwordHash[:], a.passwordHash[:])
	return a.enabled && usernameMatch&passwordMatch == 1
}

func (a *SessionAuth) signSession(expiresAt time.Time) string {
	payload := []byte(a.username + "\n" + strconv.FormatInt(expiresAt.Unix(), 10))
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(a.signature(payload))
}

func (a *SessionAuth) signature(payload []byte) []byte {
	digest := hmac.New(sha256.New, a.secret)
	_, _ = digest.Write(payload)
	return digest.Sum(nil)
}
