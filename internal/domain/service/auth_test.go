package service

import (
	"context"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAuthRepo struct {
	mock.Mock
}

func (m *MockAuthRepo) Get(ctx context.Context) (*model.AuthConfig, error) {
	args := m.Called(ctx)
	res := args.Get(0)
	if res == nil {
		return nil, args.Error(1)
	}
	return res.(*model.AuthConfig), args.Error(1)
}

func (m *MockAuthRepo) Save(ctx context.Context, auth *model.AuthConfig) error {
	args := m.Called(ctx, auth)
	return args.Error(0)
}

func (m *MockAuthRepo) Exists() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestAuthService(t *testing.T) {
	t.Run("Exists", func(t *testing.T) {
		mockRepo := new(MockAuthRepo)
		service := NewAuthService(mockRepo)
		mockRepo.On("Exists").Return(true).Once()
		assert.True(t, service.Exists())
	})

	t.Run("Create and Verify Success", func(t *testing.T) {
		mockRepo := new(MockAuthRepo)
		service := NewAuthService(mockRepo)
		ctx := context.Background()

		// Create
		mockRepo.On("Save", ctx, mock.Anything).Return(nil).Once()
		err := service.CreateCredentials(ctx, "admin", "password123")
		assert.NoError(t, err)

		// Verify
		mockRepo.On("Exists").Return(true)
		// We need the hashed password for Verify to work
		// Since we can't easily get it from the mock Save, let's just generate one for the Get mock
		hashed, _ := service.authRepo.(*MockAuthRepo).Mock.Calls[0].Arguments.Get(1).(*model.AuthConfig)
		mockRepo.On("Get", ctx).Return(hashed, nil)

		ok, err := service.Verify(ctx, "admin", "password123")
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Verify Failures", func(t *testing.T) {
		mockRepo := new(MockAuthRepo)
		service := NewAuthService(mockRepo)
		ctx := context.Background()

		// Not exists
		mockRepo.On("Exists").Return(false).Once()
		ok, _ := service.Verify(ctx, "a", "b")
		assert.False(t, ok)

		// Get Error
		mockRepo.On("Exists").Return(true)
		mockRepo.On("Get", ctx).Return((*model.AuthConfig)(nil), fmt.Errorf("error")).Once()
		_, err := service.Verify(ctx, "a", "b")
		assert.Error(t, err)

		// Nil config
		mockRepo.On("Get", ctx).Return((*model.AuthConfig)(nil), nil).Once()
		ok, err = service.Verify(ctx, "a", "b")
		assert.NoError(t, err)
		assert.False(t, ok)

		// Wrong password
		auth := &model.AuthConfig{Username: "user", Password: "hashed_password"} // invalid hash
		mockRepo.On("Get", ctx).Return(auth, nil).Once()
		ok, _ = service.Verify(ctx, "user", "pass")
		assert.False(t, ok)
	})

	t.Run("Create Errors", func(t *testing.T) {
		mockRepo := new(MockAuthRepo)
		service := NewAuthService(mockRepo)

		// Bcrypt error (password too long)
		err := service.CreateCredentials(context.Background(), "u", string(make([]byte, 100)))
		assert.Error(t, err)

		// Repo error
		mockRepo.On("Save", mock.Anything, mock.Anything).Return(fmt.Errorf("save error")).Once()
		err = service.CreateCredentials(context.Background(), "u", "p")
		assert.Error(t, err)
	})
}
