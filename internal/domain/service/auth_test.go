package service

import (
	"context"
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AuthConfig), args.Error(1)
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
	mockRepo := new(MockAuthRepo)
	service := NewAuthService(mockRepo)

	t.Run("Exists", func(t *testing.T) {
		mockRepo.On("Exists").Return(true).Once()
		assert.True(t, service.Exists())
	})

	t.Run("Create and Verify", func(t *testing.T) {
		ctx := context.Background()
		var savedAuth *model.AuthConfig
		mockRepo.On("Save", ctx, mock.Anything).Run(func(args mock.Arguments) {
			savedAuth = args.Get(1).(*model.AuthConfig)
		}).Return(nil).Once()

		err := service.CreateCredentials(ctx, "user", "pass1234")
		assert.NoError(t, err)
		assert.Equal(t, "user", savedAuth.Username)

		mockRepo.On("Exists").Return(true)
		mockRepo.On("Get", ctx).Return(savedAuth, nil)

		ok, err := service.Verify(ctx, "user", "pass1234")
		assert.NoError(t, err)
		assert.True(t, ok)

		ok, err = service.Verify(ctx, "user", "wrongpass")
		assert.NoError(t, err)
		assert.False(t, ok)

		ok, err = service.Verify(ctx, "wronguser", "pass1234")
		assert.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("Verify when not exists", func(t *testing.T) {
		mockRepo.On("Exists").Return(false).Once()
		ok, err := service.Verify(context.Background(), "user", "pass")
		assert.NoError(t, err)
		assert.False(t, ok)
	})
}
