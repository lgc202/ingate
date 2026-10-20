package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisclient "github.com/lgc202/ingate/internal/pkg/redis"
	goredis "github.com/redis/go-redis/v9"
)

func TestSessionExpiresAndCanBeDeleted(t *testing.T) {
	server := miniredis.RunT(t)
	client, err := redisclient.New(redisclient.Config{Addrs: []string{server.Addr()}})
	if err != nil {
		t.Fatalf("redisclient.New() error = %v, want nil", err)
	}

	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
	})
	store := New(client)

	want := Session{AccountID: 1, AccountVersion: 3}
	if err := store.CreateSession(context.Background(), "first", want, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession(first) error = %v, want nil", err)
	}

	got, err := store.GetSession(context.Background(), "first")
	if err != nil {
		t.Fatalf("GetSession(first) error = %v, want nil", err)
	}

	if got != want {
		t.Errorf("GetSession(first) = %+v, want %+v", got, want)
	}

	if err := store.DeleteSession(context.Background(), "first"); err != nil {
		t.Errorf("DeleteSession(first) error = %v, want nil", err)
	}

	if _, err := store.GetSession(context.Background(), "first"); !errors.Is(err, ErrNotFound) || !errors.Is(err, goredis.Nil) {
		t.Errorf("GetSession(first) after DeleteSession error = %v, want ErrNotFound and Redis nil cause", err)
	}

	if err := store.CreateSession(context.Background(), "second", want, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession(second) error = %v, want nil", err)
	}

	server.FastForward(2 * time.Hour)
	if _, err := store.GetSession(context.Background(), "second"); !errors.Is(err, ErrNotFound) || !errors.Is(err, goredis.Nil) {
		t.Errorf("GetSession(second) after expiry error = %v, want ErrNotFound and Redis nil cause", err)
	}
}
