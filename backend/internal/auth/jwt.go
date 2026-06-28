package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims for access and refresh tokens.
type Claims struct {
	jwt.RegisteredClaims
	UserID string `json:"uid"`
	Role   string `json:"role"`
}

// ElevationClaims represents a short-lived step-up token for destructive actions.
type ElevationClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"uid"`
	Action string `json:"action"` // e.g. "connector.delete"
}

// TokenPair is the response for a successful login or refresh.
type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int    `json:"expiresIn"` // seconds until access token expires
}

// ElevationToken is the response for a successful step-up re-authentication.
type ElevationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// JWTService handles JWT creation and validation.
type JWTService struct {
	secret       []byte
	accessTTL    time.Duration
	refreshTTL   time.Duration
	elevationTTL time.Duration
}

// NewJWTService creates a new JWTService.
func NewJWTService(secret string, accessTTL, refreshTTL time.Duration) *JWTService {
	return &JWTService{
		secret:       []byte(secret),
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		elevationTTL: 60 * time.Second, // hardcoded: elevation tokens live 60s
	}
}

// IssuePair creates a new access + refresh token pair.
func (s *JWTService) IssuePair(userID, role string) (*TokenPair, error) {
	now := time.Now()

	access, err := s.issue(Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			ID:        newTokenID(),
		},
		UserID: userID,
		Role:   role,
	})
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	refresh, err := s.issue(Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTTL)),
			ID:        newTokenID(),
		},
		UserID: userID,
		Role:   role,
	})
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int(s.accessTTL.Seconds()),
	}, nil
}

// ValidateAccess validates an access token and returns its claims.
func (s *JWTService) ValidateAccess(tokenString string) (*Claims, error) {
	return s.validate(tokenString, &Claims{})
}

// ValidateRefresh validates a refresh token and returns its claims.
func (s *JWTService) ValidateRefresh(tokenString string) (*Claims, error) {
	return s.validate(tokenString, &Claims{})
}

// IssueElevation creates a short-lived elevation token scoped to a single action.
func (s *JWTService) IssueElevation(userID, action string) (*ElevationToken, error) {
	now := time.Now()
	expiresAt := now.Add(s.elevationTTL)

	token, err := s.issueElevation(ElevationClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        newTokenID(),
		},
		UserID: userID,
		Action: action,
	})
	if err != nil {
		return nil, fmt.Errorf("issue elevation token: %w", err)
	}

	return &ElevationToken{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateElevation validates an elevation token and checks it matches the required action.
func (s *JWTService) ValidateElevation(tokenString, action string) (*ElevationClaims, error) {
	claims, err := s.validateElevation(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Action != action {
		return nil, fmt.Errorf("elevation token is for action %q, not %q", claims.Action, action)
	}
	return claims, nil
}

func (s *JWTService) issue(claims Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *JWTService) validate(tokenString string, claims *Claims) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	c, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return c, nil
}

func (s *JWTService) issueElevation(claims ElevationClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *JWTService) validateElevation(tokenString string) (*ElevationClaims, error) {
	claims := &ElevationClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse elevation token: %w", err)
	}
	c, ok := token.Claims.(*ElevationClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid elevation token")
	}
	return c, nil
}

// newTokenID generates a simple unique ID. In production, use UUID.
var tokenIDCounter int64

func newTokenID() string {
	tokenIDCounter++
	return fmt.Sprintf("jti_%d_%d", time.Now().UnixNano(), tokenIDCounter)
}
