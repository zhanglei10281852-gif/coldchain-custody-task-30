package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/clock"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/identity"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/storage/sqlite"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	path := envOr("DATABASE_PATH", "./data/coldchain.db")
	email := envOr("BOOTSTRAP_EMAIL", "operator@example.test")
	password := os.Getenv("BOOTSTRAP_PASSWORD")
	if len(password) < 12 {
		fmt.Fprintln(os.Stderr, "BOOTSTRAP_PASSWORD must contain at least 12 characters")
		os.Exit(2)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	now := clock.Real{}.Now()
	user := domain.User{ID: identity.New("usr"), Email: strings.ToLower(strings.TrimSpace(email)), DisplayName: "Operations", PasswordHash: string(hash), Role: domain.RoleOperations, Status: domain.UserActive, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := user.Validate(); err != nil {
		panic(err)
	}
	store, err := sqlite.Open(context.Background(), path)
	if err != nil {
		panic(err)
	}
	defer store.Close()
	err = store.WithTx(context.Background(), func(tx repository.Tx) error { return tx.InsertUser(context.Background(), user) })
	if err != nil {
		panic(err)
	}
	fmt.Printf("created %s at %s\n", user.Email, time.Now().UTC().Format(time.RFC3339))
}

func envOr(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
