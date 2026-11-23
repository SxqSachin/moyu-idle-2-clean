package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Service struct {
	secret []byte
	ttl    time.Duration
}

type Claims struct {
	UserID         int64
	SessionVersion int64
}

func New(secret string, ttl time.Duration) *Service {
	return &Service{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (s *Service) Sign(userID, sessionVersion int64) (string, error) {
	if sessionVersion <= 0 {
		sessionVersion = 1
	}
	claims := jwt.MapClaims{
		"sub": userID,
		"sv":  sessionVersion,
		"exp": time.Now().Add(s.ttl).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *Service) Parse(tokenString string) (int64, error) {
	claims, err := s.ParseWithSession(tokenString)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}

func (s *Service) ParseWithSession(tokenString string) (Claims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return Claims{}, ErrInvalidToken
	}

	rawClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, ErrInvalidToken
	}
	sub, ok := rawClaims["sub"]
	if !ok {
		return Claims{}, ErrInvalidToken
	}
	out := Claims{SessionVersion: 1}
	switch v := sub.(type) {
	case float64:
		out.UserID = int64(v)
	case int64:
		out.UserID = v
	default:
		return Claims{}, ErrInvalidToken
	}
	if rawSV, ok := rawClaims["sv"]; ok {
		switch v := rawSV.(type) {
		case float64:
			out.SessionVersion = int64(v)
		case int64:
			out.SessionVersion = v
		}
		if out.SessionVersion <= 0 {
			out.SessionVersion = 1
		}
	}
	return out, nil
}
