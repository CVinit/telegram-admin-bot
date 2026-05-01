package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/CVinit/telegram-admin-bot/internal/dujiao"
	"github.com/CVinit/telegram-admin-bot/internal/storage"
)

var (
	ErrSessionRequired = errors.New("admin session required")
	ErrSessionExpired  = errors.New("admin session expired")
)

type authAPI interface {
	Login(ctx context.Context, username, password string) (*dujiao.LoginResponse, error)
	GetAuthzMe(ctx context.Context, token string) (*dujiao.AuthzMeResponse, error)
}

type sessionStore interface {
	GetSessionByTelegramUser(ctx context.Context, telegramUser int64) (*storage.AdminSession, error)
	UpsertSession(ctx context.Context, session *storage.AdminSession) error
	DeleteSessionByTelegramUser(ctx context.Context, telegramUser int64) error
}

type Service struct {
	api      authAPI
	sessions sessionStore
	now      func() time.Time
}

type SessionView struct {
	TelegramUser int64
	AdminID      uint
	Username     string
	JWTToken     string
	JWTExpiresAt time.Time
	Roles        []string
	Policies     []dujiao.AuthzPolicy
	IsSuper      bool
	LastLoginAt  time.Time
	LastUsedAt   time.Time
}

func NewService(api *dujiao.Client, sessions *storage.SessionStore) *Service {
	return newService(api, sessions, time.Now)
}

func newService(api authAPI, sessions sessionStore, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		api:      api,
		sessions: sessions,
		now:      now,
	}
}

func (s *Service) Login(ctx context.Context, telegramUser int64, username, password string) (*SessionView, error) {
	if err := s.validateLoginRequest(telegramUser, username, password); err != nil {
		return nil, err
	}
	if s.api == nil {
		return nil, errors.New("session auth API is nil")
	}
	if s.sessions == nil {
		return nil, errors.New("session store is nil")
	}

	loginResp, err := s.api.Login(ctx, strings.TrimSpace(username), password)
	if err != nil {
		return nil, fmt.Errorf("dujiao login: %w", err)
	}
	if loginResp == nil {
		return nil, errors.New("dujiao login response is nil")
	}

	token := strings.TrimSpace(loginResp.Token)
	if token == "" {
		return nil, errors.New("dujiao login token is empty")
	}

	authzResp, err := s.api.GetAuthzMe(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("dujiao authz me: %w", err)
	}
	if authzResp == nil {
		return nil, errors.New("dujiao authz response is nil")
	}

	expiresAt, err := parseExpiresAt(loginResp.ExpiresAt)
	if err != nil {
		return nil, err
	}

	adminID := authzResp.AdminID
	if adminID == 0 {
		adminID = loginResp.User.ID
	}
	if adminID == 0 {
		return nil, errors.New("admin ID is missing from auth responses")
	}

	rolesJSON, err := json.Marshal(authzResp.Roles)
	if err != nil {
		return nil, fmt.Errorf("marshal roles: %w", err)
	}
	policiesJSON, err := json.Marshal(authzResp.Policies)
	if err != nil {
		return nil, fmt.Errorf("marshal policies: %w", err)
	}

	loginAt := s.now().UTC()
	session := &storage.AdminSession{
		TelegramUser: telegramUser,
		AdminID:      adminID,
		IsSuper:      authzResp.IsSuper,
		Username:     normalizeUsername(loginResp.User.Username, username),
		JWTToken:     token,
		JWTExpiresAt: expiresAt,
		RolesJSON:    string(rolesJSON),
		PoliciesJSON: string(policiesJSON),
		LastLoginAt:  loginAt,
		LastUsedAt:   loginAt,
	}
	if err := s.sessions.UpsertSession(ctx, session); err != nil {
		return nil, fmt.Errorf("upsert session: %w", err)
	}

	view, err := sessionToView(session)
	if err != nil {
		return nil, err
	}
	view.IsSuper = authzResp.IsSuper
	return view, nil
}

func (s *Service) RequireSession(ctx context.Context, telegramUser int64) (*SessionView, error) {
	if telegramUser <= 0 {
		return nil, errors.New("telegram user must be positive")
	}
	if s.sessions == nil {
		return nil, errors.New("session store is nil")
	}

	session, err := s.sessions.GetSessionByTelegramUser(ctx, telegramUser)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	if session == nil {
		return nil, ErrSessionRequired
	}
	if session.JWTExpiresAt.IsZero() || !session.JWTExpiresAt.After(s.now().UTC()) {
		return nil, ErrSessionExpired
	}

	return sessionToView(session)
}

func (s *Service) Logout(ctx context.Context, telegramUser int64) error {
	if telegramUser <= 0 {
		return errors.New("telegram user must be positive")
	}
	if s.sessions == nil {
		return errors.New("session store is nil")
	}
	return s.sessions.DeleteSessionByTelegramUser(ctx, telegramUser)
}

func (s *Service) validateLoginRequest(telegramUser int64, username, password string) error {
	if telegramUser <= 0 {
		return errors.New("telegram user must be positive")
	}
	if strings.TrimSpace(username) == "" {
		return errors.New("username is required")
	}
	if password == "" {
		return errors.New("password is required")
	}
	return nil
}

func sessionToView(session *storage.AdminSession) (*SessionView, error) {
	if session == nil {
		return nil, errors.New("session is nil")
	}

	roles, err := decodeRoles(session.RolesJSON)
	if err != nil {
		return nil, err
	}
	policies, err := decodePolicies(session.PoliciesJSON)
	if err != nil {
		return nil, err
	}

	return &SessionView{
		TelegramUser: session.TelegramUser,
		AdminID:      session.AdminID,
		IsSuper:      session.IsSuper,
		Username:     session.Username,
		JWTToken:     session.JWTToken,
		JWTExpiresAt: session.JWTExpiresAt,
		Roles:        roles,
		Policies:     policies,
		LastLoginAt:  session.LastLoginAt,
		LastUsedAt:   session.LastUsedAt,
	}, nil
}

func decodeRoles(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var roles []string
	if err := json.Unmarshal([]byte(raw), &roles); err != nil {
		return nil, fmt.Errorf("decode roles JSON: %w", err)
	}
	return roles, nil
}

func decodePolicies(raw string) ([]dujiao.AuthzPolicy, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var policies []dujiao.AuthzPolicy
	if err := json.Unmarshal([]byte(raw), &policies); err != nil {
		return nil, fmt.Errorf("decode policies JSON: %w", err)
	}
	return policies, nil
}

func parseExpiresAt(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("dujiao login expires_at is empty")
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05Z07:00",
	}
	for _, layout := range layouts {
		ts, err := time.Parse(layout, raw)
		if err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse expires_at %q: unsupported format", raw)
}

func normalizeUsername(apiUsername, fallback string) string {
	apiUsername = strings.TrimSpace(apiUsername)
	if apiUsername != "" {
		return apiUsername
	}
	return strings.TrimSpace(fallback)
}
