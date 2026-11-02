package service

import (
	"context"
	"crypto/subtle"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"
	"log/slog"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	authRepo ports.AuthPort
	logger   *slog.Logger
}

func NewAuthService(authRepo ports.AuthPort, logger *slog.Logger) *AuthService {
	return &AuthService{authRepo: authRepo, logger: logger}
}

func (s *AuthService) Verify(ctx context.Context, username, password string) (bool, error) {
	if !s.authRepo.Exists() {
		return false, nil
	}

	config, err := s.authRepo.Get(ctx)
	if err != nil {
		return false, err
	}
	if config == nil {
		return false, nil
	}

	userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(config.Username)) == 1
	if bcrypt.CompareHashAndPassword([]byte(config.Password), []byte(password)) != nil {
		s.logger.Warn("Auth: invalid password", "username", username)
		return false, nil
	}
	if !userMatch {
		s.logger.Warn("Auth: unknown username", "username", username)
	}

	return userMatch, nil
}

func (s *AuthService) CreateCredentials(ctx context.Context, username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	auth := &model.AuthConfig{
		Username: username,
		Password: string(hashedPassword),
	}

	s.logger.Debug("Auth: credentials created", "username", username)
	return s.authRepo.Save(ctx, auth)
}

func (s *AuthService) Exists() bool {
	return s.authRepo.Exists()
}
