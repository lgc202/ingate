package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lgc202/ingate/internal/console/conf"
)

const (
	sessionCookieName   = "ingate_session"
	forwardedUserHeader = "X-Forwarded-User"
	maxLoginBodyBytes   = 16 << 10
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sessionResponse struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username"`
}

// SessionAuth 为单管理员控制台签发和校验无状态会话
// 它只解决开源单实例的管理入口保护，不建立用户、角色或权限模型
type SessionAuth struct {
	enabled      bool
	username     string
	passwordHash [sha256.Size]byte
	secret       []byte
	ttl          time.Duration
	secureCookie bool
}

// NewSessionAuth 根据 Console 配置创建管理面会话认证
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

// HandleSession 提供登录状态查询、登录和退出三个同源接口
func (a *SessionAuth) HandleSession(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		a.writeCurrentSession(response, request)
	case http.MethodPost:
		a.login(response, request)
	case http.MethodDelete:
		a.logout(response)
	default:
		response.Header().Set("Allow", "GET, POST, DELETE")
		a.writeError(response, http.StatusMethodNotAllowed, "请求方法不支持")
	}
}

// Protect 验证管理 API 会话，并把可信管理员身份传给内部 Admin API
func (a *SessionAuth) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		request.Header.Del(forwardedUserHeader)
		username, ok := a.currentUser(request)
		if !ok {
			a.writeError(response, http.StatusUnauthorized, "登录状态已失效，请重新登录")
			return
		}
		request.Header.Set(forwardedUserHeader, username)
		next.ServeHTTP(response, request)
	})
}

func (a *SessionAuth) writeCurrentSession(response http.ResponseWriter, request *http.Request) {
	username, authenticated := a.currentUser(request)
	a.writeResponse(response, http.StatusOK, sessionResponse{
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
		a.writeError(response, http.StatusBadRequest, "登录信息格式不正确")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		a.writeError(response, http.StatusBadRequest, "登录信息格式不正确")
		return
	}
	if !a.validCredentials(credentials) {
		a.writeError(response, http.StatusUnauthorized, "用户名或密码不正确")
		return
	}

	expiresAt := time.Now().Add(a.ttl)
	http.SetCookie(response, &http.Cookie{
		Name:     sessionCookieName,
		Value:    a.signSession(expiresAt),
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(a.ttl.Seconds()),
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	a.writeResponse(response, http.StatusOK, sessionResponse{Authenticated: true, Username: a.username})
}

func (a *SessionAuth) logout(response http.ResponseWriter) {
	http.SetCookie(response, &http.Cookie{
		Name:     sessionCookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	a.writeResponse(response, http.StatusOK, sessionResponse{})
}

func (a *SessionAuth) currentUser(request *http.Request) (string, bool) {
	if !a.enabled {
		return a.username, true
	}
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
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

func (a *SessionAuth) writeResponse(response http.ResponseWriter, status int, data sessionResponse) {
	_ = writeJSON(response, status, map[string]any{
		"code": status,
		"msg":  "",
		"data": data,
	})
}

func (a *SessionAuth) writeError(response http.ResponseWriter, status int, message string) {
	_ = writeJSON(response, status, map[string]any{
		"code": status,
		"msg":  message,
		"data": nil,
	})
}
