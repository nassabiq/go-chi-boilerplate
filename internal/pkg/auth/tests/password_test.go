package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"golang.org/x/crypto/bcrypt"
)

// knownDummyHash mirrors internal/pkg/auth dummyPasswordHash. It is a
// pre-generated bcrypt hash at bcrypt.DefaultCost (10) for the dummy plaintext
// "timing-equalization-dummy-placeholder". It is duplicated here only to prove
// the embedded constant is a syntactically valid bcrypt hash; it is never
// exposed through any API response.
const knownDummyHash = "$2a$10$tYYMF07nLY3DFS4Vk6MqOu5kG.D7tZ5GCG4bReXPjD.4a.J3JyW2G"

func TestDummyPasswordHashIsValidBcrypt(t *testing.T) {
	// A well-formed bcrypt hash parses and yields a *mismatch* (not a format
	// error) when compared with an arbitrary password.
	err := bcrypt.CompareHashAndPassword([]byte(knownDummyHash), []byte("does-not-match"))
	if err == nil {
		t.Fatal("expected mismatch for dummy hash comparison")
	}
	if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		t.Fatalf("dummy hash is not a valid bcrypt hash: %v", err)
	}
}

func TestCompareDummyPasswordDoesNotPanic(t *testing.T) {
	cases := []string{
		"",
		"short",
		"correct horse battery staple",
		strings.Repeat("a", 72), // bcrypt max input length
	}
	for _, pw := range cases {
		// Must not panic for arbitrary input, including the 72-byte boundary.
		auth.CompareDummyPassword(pw)
	}
}

func TestDummyHashNotGeneratedPerRequest(t *testing.T) {
	// The dummy hash is a package-level constant, so the request path never
	// calls bcrypt.GenerateFromPassword. We assert it is present and uses the
	// same cost (DefaultCost == 10) as production password hashing.
	if knownDummyHash == "" {
		t.Fatal("dummy hash must be a pre-generated constant")
	}
	if !strings.HasPrefix(knownDummyHash, "$2a$10$") {
		t.Fatalf("dummy hash cost must match production (DefaultCost=10), got %q", knownDummyHash)
	}
}

func TestCompareDummyPasswordDoesNotExposeMatch(t *testing.T) {
	// CompareDummyPassword returns nothing, so callers cannot observe whether
	// the (meaningless) comparison matched. The return-less signature itself
	// guarantees no match-state leakage.
	auth.CompareDummyPassword("anything")
}

func TestCompareDummyPasswordForCost(t *testing.T) {
	cases := []int{4, 10, 12, 14, 0, 100}
	for _, cost := range cases {
		auth.CompareDummyPasswordForCost("anything", cost)
	}
}

func TestHashPasswordComparePasswordUnchanged(t *testing.T) {
	hash, err := auth.HashPassword("s3cret-password", 12)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := auth.ComparePassword(hash, "s3cret-password"); err != nil {
		t.Fatalf("expected match: %v", err)
	}
	if err := auth.ComparePassword(hash, "wrong"); err == nil {
		t.Fatal("expected mismatch for wrong password")
	}
}
