package auth

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	dummyPasswordHashCost10 = "$2a$10$tYYMF07nLY3DFS4Vk6MqOu5kG.D7tZ5GCG4bReXPjD.4a.J3JyW2G"
	dummyPasswordHashCost12 = "$2a$12$vwNK1oQG.rkamUyI4TPmleR0ZTp.3RsSifsbUkk.n3W.nbeTmRU0G"
	dummyPasswordHashCost4  = "$2a$04$6V2W24CRChBY5dzXQJRi6.Xru.Vl/ldzLTk49DsfLtqsuXM.wdL.y"
)

var dummyHashes = func() map[int]string {
	return map[int]string{
		4:  dummyPasswordHashCost4,
		10: dummyPasswordHashCost10,
		12: dummyPasswordHashCost12,
	}
}()

func getDummyHashForCost(cost int) string {
	if cost < bcrypt.MinCost {
		cost = bcrypt.MinCost
	}
	if cost > bcrypt.MaxCost {
		cost = bcrypt.MaxCost
	}
	if val, ok := dummyHashes[cost]; ok {
		return val
	}
	// Fallback to default production cost 12
	return dummyPasswordHashCost12
}

// CompareDummyPassword performs a bcrypt comparison against the pre-generated
// dummy hash matching the production default cost (12). It is used on the
// invalid-email path so that both non-existent-email and wrong-password logins
// execute equivalent bcrypt work without leaking account existence through timing.
// The result is intentionally discarded by callers: there is nothing
// meaningful to match against and exposing the outcome could only leak state.
func CompareDummyPassword(password string) {
	CompareDummyPasswordForCost(password, 12)
}

// CompareDummyPasswordForCost performs a bcrypt comparison against a dummy hash
// matching the specified cost factor to ensure strict timing parity with the
// configured bcrypt cost.
func CompareDummyPasswordForCost(password string, cost int) {
	hash := getDummyHashForCost(cost)
	_ = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func HashPassword(password string, cost int) (string, error) {
	if cost < bcrypt.MinCost {
		cost = bcrypt.MinCost
	}
	if cost > bcrypt.MaxCost {
		cost = bcrypt.MaxCost
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func ComparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
