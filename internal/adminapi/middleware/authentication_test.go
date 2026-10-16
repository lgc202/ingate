package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	goredis "github.com/redis/go-redis/v9"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	mysqlstore "github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	redisstore "github.com/lgc202/ingate/internal/adminapi/repository/redis"
	accountservice "github.com/lgc202/ingate/internal/adminapi/service/account"
)

func TestRequireSession(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	digest := sha256.Sum256([]byte(token))
	id := hex.EncodeToString(digest[:])

	for _, tt := range []struct {
		name           string
		header         string
		createSession  bool
		accountVersion uint64
		redisFailure   bool
		expired        bool
		wantStatus     int
	}{
		{
			name:           "valid",
			header:         "Bearer " + token,
			createSession:  true,
			accountVersion: 7,
			wantStatus:     http.StatusOK,
		},
		{name: "missing bearer", wantStatus: http.StatusUnauthorized},
		{name: "malformed token", header: "Bearer invalid", wantStatus: http.StatusUnauthorized},
		{name: "unknown token", header: "Bearer " + token, wantStatus: http.StatusUnauthorized},
		{
			name:           "revoked session",
			header:         "Bearer " + token,
			createSession:  true,
			accountVersion: 8,
			wantStatus:     http.StatusUnauthorized,
		},
		{
			name:          "expired session",
			header:        "Bearer " + token,
			createSession: true,
			expired:       true,
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:         "Redis unavailable",
			header:       "Bearer " + token,
			redisFailure: true,
			wantStatus:   http.StatusServiceUnavailable,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}

			t.Cleanup(func() {
				mock.ExpectClose()
				if err := db.Close(); err != nil {
					t.Error(err)
				}

				if err := mock.ExpectationsWereMet(); err != nil {
					t.Error(err)
				}
			})
			gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
				Conn:                      db,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}

			redisServer := miniredis.RunT(t)
			client := goredis.NewClient(&goredis.Options{Addr: redisServer.Addr()})
			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			})
			sessions := repository.NewSession(redisstore.New(client))
			if tt.createSession {
				if err := sessions.Create(t.Context(), id, domain.Session{
					AccountID:      1,
					AccountVersion: 7,
				}, time.Now().Add(time.Hour)); err != nil {
					t.Fatal(err)
				}

				if tt.expired {
					redisServer.FastForward(2 * time.Hour)
				} else {
					mock.ExpectQuery("SELECT .* FROM `accounts`").WithArgs(uint64(1), 1).
						WillReturnRows(sqlmock.NewRows([]string{"id", "session_version"}).AddRow(1, tt.accountVersion))
				}
			}

			if tt.redisFailure {
				redisServer.SetError("test Redis failure")
			}

			service := accountservice.New(repository.NewAccount(mysqlstore.New(gormDB)), sessions)
			server := kratoshttp.NewServer(kratoshttp.Middleware(RequireSession(service)))
			calls := 0
			server.Route("/").GET("/session", func(ctx kratoshttp.Context) error {
				handler := ctx.Middleware(func(ctx context.Context, _ any) (any, error) {
					calls++
					identity, ok := SessionFromContext(ctx)
					if !ok || identity.ID != id || identity.AccountID != 1 {
						t.Errorf("identity = %+v, present = %t", identity, ok)
					}

					return nil, nil
				})
				if _, err := handler(ctx, nil); err != nil {
					return err
				}

				return ctx.Result(http.StatusOK, map[string]string{"status": "ok"})
			})

			request := httptest.NewRequest(http.MethodGet, "/session", nil)
			request.Header.Set("Authorization", tt.header)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", response.Code, tt.wantStatus, response.Body.String())
			}

			wantCalls := 0
			if tt.wantStatus == http.StatusOK {
				wantCalls = 1
			}

			if calls != wantCalls {
				t.Errorf("downstream calls = %d, want %d", calls, wantCalls)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		wantToken     string
		wantOK        bool
	}{
		{name: "standard", authorization: "Bearer abc", wantToken: "abc", wantOK: true},
		{name: "case insensitive", authorization: "bearer abc", wantToken: "abc", wantOK: true},
		{name: "missing", authorization: "Bearer", wantOK: false},
		{name: "wrong scheme", authorization: "Basic abc", wantOK: false},
		{name: "extra field", authorization: "Bearer abc def", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := bearerToken(tt.authorization)
			if token != tt.wantToken || ok != tt.wantOK {
				t.Errorf("bearerToken(%q) = (%q, %t), want (%q, %t)", tt.authorization, token, ok, tt.wantToken, tt.wantOK)
			}
		})
	}
}
