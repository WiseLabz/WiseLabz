// Package auth provides JWT token issuance, validation, and HTTP middleware.
package auth

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	tokenAudienceAccess    = "access"
	tokenAudienceRefresh   = "refresh"
	tokenAudienceElevation = "elevation"
	tokenAudienceMFA       = "mfa"
)

// Claims represents the JWT claims for access and refresh tokens.
// InstanceAdmin reflects only the flat, non-connector-scoped role (user
// management, API keys, granting connector permissions); per-connector
// access is looked up per-request from the store, not carried in the token,
// since it can change per connector at any time (see auth.RequireConnectorRole).
//
// MFAEnrollOnly marks a session issued to a user the require_2fa policy
// covers but who hasn't enrolled a factor yet (#279). AuthMiddleware confines
// such a session to the enrollment allowlist; the refresh path carries the
// flag forward until ConfirmFactor drops it.
type Claims struct {
	jwt.RegisteredClaims
	UserID        string `json:"uid"`
	InstanceAdmin bool   `json:"admin"`
	MFAEnrollOnly bool   `json:"mfa_enroll,omitempty"`
	// SessionID identifies the login session across access/refresh rotation;
	// elevation tokens are bound to it. Empty on tokens minted before it existed.
	SessionID string `json:"sid,omitempty"`
}

// MFAClaims represents a short-lived ticket issued after a correct password
// but before the second factor, identifying only who is completing login.
type MFAClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"uid"`
}

// MFATicket is the response for a login that requires a second factor.
type MFATicket struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// IssuePairOptions carries per-issuance flags for IssuePairWithOptions.
type IssuePairOptions struct {
	// MFAEnrollOnly stamps both tokens of the pair with the enrollment-only
	// claim (see Claims.MFAEnrollOnly).
	MFAEnrollOnly bool
	// SessionID continues an existing session (refresh); empty starts a new one.
	SessionID string
}

// APIKeyClaims represents the identity and lifecycle fields needed to
// authenticate an opaque API key. Like Claims.InstanceAdmin, InstanceAdmin
// here is the flat instance-wide role only.
type APIKeyClaims struct {
	KeyID         string
	UserID        string
	InstanceAdmin bool
	ExpiresAt     string
	RevokedAt     string
	LastUsedAt    string
	// Restriction narrows what the key can do below its owner's own access
	// (#278). The zero value is an unrestricted key.
	Restriction APIKeyRestriction
}

// ElevationClaims represents a short-lived step-up token for destructive actions.
type ElevationClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"uid"`
	Action string `json:"action"` // e.g. "connector.delete"
	// SessionID and Target bind the token to the session that earned it and to
	// the one resource it was issued for ("" for actions with no target).
	SessionID string `json:"sid,omitempty"`
	Target    string `json:"tgt,omitempty"`
}

// ElevationBinding narrows an elevation token to one session and one target.
type ElevationBinding struct {
	SessionID string
	Target    string
}

// TokenPair is the response for a successful login or refresh.
type TokenPair struct {
	// SessionID is the session the pair belongs to; callers persist it as the
	// session row's ID so refresh rotation keeps it stable.
	SessionID    string `json:"-"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int    `json:"expiresIn"` // seconds until access token expires
}

// ElevationToken is the response for a successful step-up re-authentication.
type ElevationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Service handles JWT creation and validation.
type Service struct {
	secret       []byte
	accessTTL    time.Duration
	refreshTTL   time.Duration
	elevationTTL time.Duration
	settings     func() (RuntimeSettings, bool)

	usedMu      sync.Mutex
	usedElevate map[string]time.Time // spent elevation jti -> token expiry
}

// RuntimeSettings are the operator-editable auth settings that override the
// static config TTLs and the step-up default.
type RuntimeSettings struct {
	AccessTTL            time.Duration
	RefreshTTL           time.Duration
	StepUpForDestructive bool
}

// SetSettingsSource installs a provider of runtime settings (the persisted
// auth_config). Call once at startup, before serving. When the provider
// reports !ok or a non-positive TTL, the constructor values apply.
func (s *Service) SetSettingsSource(fn func() (RuntimeSettings, bool)) { s.settings = fn }

func (s *Service) ttls() (access, refresh time.Duration) {
	access, refresh = s.accessTTL, s.refreshTTL
	if s.settings != nil {
		if rs, ok := s.settings(); ok {
			if rs.AccessTTL > 0 {
				access = rs.AccessTTL
			}
			if rs.RefreshTTL > 0 {
				refresh = rs.RefreshTTL
			}
		}
	}
	return access, refresh
}

// RefreshTTL returns the refresh-token lifetime currently in force.
func (s *Service) RefreshTTL() time.Duration {
	_, refresh := s.ttls()
	return refresh
}

// StepUpEnabled reports whether destructive actions require step-up. It
// defaults to true when no settings source is installed.
func (s *Service) StepUpEnabled() bool {
	if s.settings != nil {
		if rs, ok := s.settings(); ok {
			return rs.StepUpForDestructive
		}
	}
	return true
}

// NewService creates a new Service.
func NewService(secret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		secret:       []byte(secret),
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		elevationTTL: 60 * time.Second, // hardcoded: elevation tokens live 60s
	}
}

// IssuePair creates a new access + refresh token pair.
func (s *Service) IssuePair(userID string, instanceAdmin bool) (*TokenPair, error) {
	return s.IssuePairWithOptions(userID, instanceAdmin, IssuePairOptions{})
}

// IssuePairWithOptions creates a new access + refresh token pair, applying
// opts to both tokens.
func (s *Service) IssuePairWithOptions(userID string, instanceAdmin bool, opts IssuePairOptions) (*TokenPair, error) {
	now := time.Now()
	accessTTL, refreshTTL := s.ttls()
	sessionID := opts.SessionID
	if sessionID == "" {
		sessionID = newTokenID()
	}

	access, err := s.issue(Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{tokenAudienceAccess},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
			ID:        newTokenID(),
		},
		UserID:        userID,
		InstanceAdmin: instanceAdmin,
		MFAEnrollOnly: opts.MFAEnrollOnly,
		SessionID:     sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	refresh, err := s.issue(Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{tokenAudienceRefresh},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTTL)),
			ID:        newTokenID(),
		},
		UserID:        userID,
		InstanceAdmin: instanceAdmin,
		MFAEnrollOnly: opts.MFAEnrollOnly,
		SessionID:     sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	return &TokenPair{
		SessionID:    sessionID,
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int(accessTTL.Seconds()),
	}, nil
}

// IssueMFATicket creates a short-lived (5 minute) ticket identifying a user
// who passed the password step of login and now must present a second
// factor to POST /auth/login/mfa. Its "mfa" audience keeps it from being
// accepted anywhere an access, refresh or elevation token is expected, and
// vice versa (see ValidateAccess/ValidateRefresh/ValidateElevation).
func (s *Service) IssueMFATicket(userID string) (*MFATicket, error) {
	now := time.Now()
	expiresAt := now.Add(5 * time.Minute)

	token, err := s.issueMFA(MFAClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{tokenAudienceMFA},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        newTokenID(),
		},
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("issue mfa ticket: %w", err)
	}

	return &MFATicket{Ticket: token, ExpiresAt: expiresAt}, nil
}

// ValidateMFATicket validates a login MFA ticket and returns its claims.
func (s *Service) ValidateMFATicket(tokenString string) (*MFAClaims, error) {
	claims := &MFAClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse mfa ticket: %w", err)
	}
	c, ok := token.Claims.(*MFAClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid mfa ticket")
	}
	if !hasAudience(c.Audience, tokenAudienceMFA) {
		return nil, fmt.Errorf("token is not an mfa ticket")
	}
	return c, nil
}

func (s *Service) issueMFA(claims MFAClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateAccess validates an access token and returns its claims.
func (s *Service) ValidateAccess(tokenString string) (*Claims, error) {
	return s.validate(tokenString, tokenAudienceAccess)
}

// ValidateRefresh validates a refresh token and returns its claims.
func (s *Service) ValidateRefresh(tokenString string) (*Claims, error) {
	return s.validate(tokenString, tokenAudienceRefresh)
}

// IssueElevation creates a short-lived elevation token scoped to a single action.
func (s *Service) IssueElevation(userID, action string) (*ElevationToken, error) {
	return s.IssueElevationBound(userID, action, ElevationBinding{})
}

// IssueElevationBound is IssueElevation with the token bound to a session and target.
func (s *Service) IssueElevationBound(userID, action string, b ElevationBinding) (*ElevationToken, error) {
	now := time.Now()
	expiresAt := now.Add(s.elevationTTL)

	token, err := s.issueElevation(ElevationClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{tokenAudienceElevation},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        newTokenID(),
		},
		UserID:    userID,
		Action:    action,
		SessionID: b.SessionID,
		Target:    b.Target,
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
func (s *Service) ValidateElevation(tokenString, action, userID string) (*ElevationClaims, error) {
	claims, err := s.validateElevation(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Action != action {
		return nil, fmt.Errorf("elevation token is for action %q, not %q", claims.Action, action)
	}
	if claims.UserID != userID {
		return nil, fmt.Errorf("elevation token belongs to a different user")
	}
	return claims, nil
}

// ConsumeElevation is ValidateElevation plus the checks that make a token
// spendable once: it must carry the caller's session and the request's target,
// and its jti is recorded so a replay inside the TTL is refused. The token is
// only spent after every other check passed.
func (s *Service) ConsumeElevation(tokenString, action, userID string, b ElevationBinding) (*ElevationClaims, error) {
	claims, err := s.ValidateElevation(tokenString, action, userID)
	if err != nil {
		return nil, err
	}
	if claims.SessionID != b.SessionID {
		return nil, fmt.Errorf("elevation token belongs to a different session")
	}
	if claims.Target != b.Target {
		return nil, fmt.Errorf("elevation token is for a different target")
	}

	now := time.Now()
	s.usedMu.Lock()
	defer s.usedMu.Unlock()
	if s.usedElevate == nil {
		s.usedElevate = make(map[string]time.Time)
	}
	for id, exp := range s.usedElevate {
		if now.After(exp) {
			delete(s.usedElevate, id)
		}
	}
	if _, spent := s.usedElevate[claims.ID]; spent {
		return nil, fmt.Errorf("elevation token already used")
	}
	exp := now.Add(s.elevationTTL)
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	}
	s.usedElevate[claims.ID] = exp
	return claims, nil
}

func (s *Service) issue(claims Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *Service) validate(tokenString, audience string) (*Claims, error) {
	claims := &Claims{}
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
	if !hasAudience(c.Audience, audience) {
		return nil, fmt.Errorf("token is not for %s", audience)
	}
	return c, nil
}

func (s *Service) issueElevation(claims ElevationClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *Service) validateElevation(tokenString string) (*ElevationClaims, error) {
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
	if !hasAudience(c.Audience, tokenAudienceElevation) {
		return nil, fmt.Errorf("token is not an elevation token")
	}
	return c, nil
}

func hasAudience(audiences jwt.ClaimStrings, audience string) bool {
	for _, value := range audiences {
		if value == audience {
			return true
		}
	}
	return false
}

func newTokenID() string {
	return uuid.NewString()
}
