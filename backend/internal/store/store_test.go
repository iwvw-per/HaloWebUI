package store

import (
	"context"
	"errors"
	"testing"
)

func TestCreateFirstUserAsAdminAndPreserveSchema(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	user, err := store.CreateUser(
		ctx, "user-1", "Admin", "ADMIN@example.com", "hash", "/user.png", "pending",
	)
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != "admin" || user.Email != "admin@example.com" {
		t.Fatalf("unexpected first user: %#v", user)
	}

	_, err = store.CreateUser(
		ctx, "user-2", "Duplicate", "admin@example.com", "hash", "/user.png", "pending",
	)
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
}
