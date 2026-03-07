package service

import (
	"context"
	"crypto/subtle"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	authRepo ports.AuthPort
}

func NewAuthService(authRepo ports.AuthPort) *AuthService {
	return &AuthService{authRepo: authRepo}
}

func (s *AuthService) Verify(ctx context.Context, username, password string) (bool, error) {
	if !s.authRepo.Exists() {
		return false, nil
	}

	config, err := s.authRepo.Get(ctx)
	if err != nil {
		return false, err
	}

	userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(config.Username)) == 1
	err = bcrypt.CompareHashAndPassword([]byte(config.Password), []byte(password))
	if err != nil {
		return false, nil
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

	return s.authRepo.Save(ctx, auth)
}

func (s *AuthService) Exists() bool {
	return s.authRepo.Exists()
}
