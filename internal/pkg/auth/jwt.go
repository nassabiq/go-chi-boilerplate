package auth

import (
	"crypto/rand"
	"encoding/base64"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
)

type JWT struct {
	accessSecret  []byte
	refreshSecret []byte
	issuer        string
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

type Claims struct {
	UserID      int64    `json:"user_id"`
	Type        string   `json:"type"`
	SessionID   string   `json:"sid"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

func NewJWT(cfg *config.Config) *JWT {
	return &JWT{
		accessSecret:  []byte(cfg.Auth.JWT.AccessSecret),
		refreshSecret: []byte(cfg.Auth.JWT.RefreshSecret),
		issuer:        cfg.Auth.JWT.Issuer,
		accessTTL:     cfg.Auth.JWT.AccessExpiry,
		refreshTTL:    cfg.Auth.JWT.RefreshExpiry,
	}
}

func (j *JWT) generate(userID int64, sessionId string, tokenType string, secret []byte, ttl time.Time, roles []string, permissions []string) (string, error) {

	tokenID, err := RandomSessionID()
	if err != nil {
		return "", apperror.New(errconstant.ErrCodeInternal, err)
	}

	claims := Claims{
		UserID:      userID,
		SessionID:   sessionId,
		Type:        tokenType,
		Roles:       roles,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			Issuer:    j.issuer,
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(ttl),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	result, err := token.SignedString(secret)
	if err != nil {
		return "", apperror.New(errconstant.ErrCodeInternal, err)
	}

	return result, nil
}

func (j *JWT) GenerateAccessToken(userID int64, sessionID string, roles []string, permissions []string) (string, error) {
	return j.generate(userID, sessionID, "access", j.accessSecret, time.Now().Add(j.accessTTL), roles, permissions)
}

func (j *JWT) GenerateRefreshToken(userID int64, sessionID string, expiresAt time.Time, roles []string, permissions []string) (string, error) {
	return j.generate(userID, sessionID, "refresh", j.refreshSecret, expiresAt, roles, permissions)
}

func (j *JWT) AccessExpiresIn() int64 {
	return int64(j.accessTTL.Seconds())
}

func (j *JWT) parse(tokenString string, tokenType string, secret []byte) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) {
			return secret, nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithIssuer(j.issuer),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil)
	}

	if !token.Valid {
		return nil, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil)
	}

	if claims.Type != tokenType {
		return nil, apperror.New(errconstant.ErrCodeAuthInvalidTokenType, nil)
	}

	if claims.UserID <= 0 {
		return nil, apperror.New(errconstant.ErrCodeInvalidParameter, nil)
	}

	return claims, nil
}

func (j *JWT) ParseAccessToken(token string) (*Claims, error) {
	return j.parse(token, "access", j.accessSecret)
}

func (j *JWT) ParseRefreshToken(token string) (*Claims, error) {
	return j.parse(token, "refresh", j.refreshSecret)
}

func RandomSessionID() (string, error) {
	value := make([]byte, 32)

	if _, err := rand.Read(value); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (j *JWT) RefreshTTL() time.Duration {
	return j.refreshTTL
}
