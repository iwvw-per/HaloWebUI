package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
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

func TestListQueriesWorkWithSingleDatabaseConnection(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	user, err := store.CreateUser(ctx, "user-1", "Admin", "admin@example.com", "hash", "/user.png", "pending")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateChat(ctx, Chat{ID: "chat-1", UserID: user.ID, Chat: json.RawMessage(`{"messages":[]}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.UpsertModel(ctx, Model{ID: "model-1", UserID: user.ID, Name: "Model", IsActive: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.PutResource(ctx, Resource{Kind: "tool", ID: "tool-1", UserID: user.ID, Key: "tool", Active: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateFile(ctx, File{ID: "file-1", UserID: user.ID, Filename: "file.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateKnowledge(ctx, Knowledge{ID: "knowledge-1", UserID: user.ID, Name: "Knowledge"}); err != nil {
		t.Fatal(err)
	}
	channel, err := store.CreateChannel(ctx, Channel{ID: "channel-1", UserID: user.ID, Name: "Channel"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateChannelMessage(ctx, ChannelMessage{ID: "message-1", UserID: user.ID, ChannelID: channel.ID, Content: "Hello"}); err != nil {
		t.Fatal(err)
	}

	store.db.SetMaxOpenConns(1)
	store.db.SetMaxIdleConns(1)
	testCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	checks := []struct {
		name string
		list func() error
	}{
		{"users", func() error { _, err := store.ListUsers(testCtx, "", 20); return err }},
		{"chats", func() error { _, err := store.ListChats(testCtx, user.ID, nil, 1, 20); return err }},
		{"models", func() error { _, err := store.ListModels(testCtx, false); return err }},
		{"resources", func() error { _, err := store.ListResources(testCtx, "tool", false); return err }},
		{"files", func() error { _, err := store.ListFiles(testCtx, user.ID); return err }},
		{"knowledge", func() error { _, err := store.ListKnowledge(testCtx, ""); return err }},
		{"channels", func() error { _, err := store.ListChannels(testCtx); return err }},
		{"channel messages", func() error { _, err := store.ListChannelMessages(testCtx, channel.ID, nil, 0, 20); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.list(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
