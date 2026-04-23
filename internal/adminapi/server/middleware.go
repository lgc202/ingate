package server

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
)

const (
	authorizationHeader   = "Authorization"
	bearerTokenPrefix     = "Bearer "
	originHeader          = "Origin"
	varyHeader            = "Vary"
	allowOriginHeader     = "Access-Control-Allow-Origin"
	allowMethodsHeader    = "Access-Control-Allow-Methods"
	allowHeadersHeader    = "Access-Control-Allow-Headers"
	maxAgeHeader          = "Access-Control-Max-Age"
	requestIDHeader       = "X-Request-ID"
	errorCodeUnauthorized = "Unauthorized"
)

var requestIDCounter uint64

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if strings.TrimSpace(requestID) == "" {
			requestID = newRequestID()
		}
		c.Header(requestIDHeader, requestID)
		c.Next()
	}
}

func bearerTokenAuthMiddleware(adminToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !validBearerToken(c.GetHeader(authorizationHeader), adminToken) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
				Code:    errorCodeUnauthorized,
				Message: "missing or invalid bearer token",
			})
			return
		}
		c.Next()
	}
}

func localDevelopmentCORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader(originHeader)
		if isLocalDevelopmentOrigin(origin) {
			c.Header(allowOriginHeader, origin)
			c.Header(varyHeader, originHeader)
			c.Header(allowMethodsHeader, "GET,POST,PUT,DELETE,OPTIONS")
			c.Header(allowHeadersHeader, "Authorization,Content-Type,Accept,X-Request-ID")
			c.Header(maxAgeHeader, "600")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func isLocalDevelopmentOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := parsed.Hostname()
	return host == "127.0.0.1" || host == "localhost"
}

func validBearerToken(headerValue, expectedToken string) bool {
	if expectedToken == "" || !strings.HasPrefix(headerValue, bearerTokenPrefix) {
		return false
	}
	actualToken := strings.TrimSpace(strings.TrimPrefix(headerValue, bearerTokenPrefix))
	return subtle.ConstantTimeCompare([]byte(actualToken), []byte(expectedToken)) == 1
}

func newRequestID() string {
	sequence := atomic.AddUint64(&requestIDCounter, 1)
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), sequence)
}
