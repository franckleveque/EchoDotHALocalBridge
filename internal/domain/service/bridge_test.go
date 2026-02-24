package service

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/amimof/huego"
)

type MockHAPort struct {
	mock.Mock
}

func (m *MockHAPort) GetDevices(ctx context.Context) ([]*model.Device, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*model.Device), args.Error(1)
}

func (m *MockHAPort) SetState(ctx context.Context, device *model.Device, params map[string]interface{}) error {
	args := m.Called(ctx, device, params)
	return args.Error(0)
}

func (m *MockHAPort) Configure(url, token string) {
	m.Called(url, token)
}

func (m *MockHAPort) IsConfigured() bool {
	args := m.Called()
	return args.Bool(0)
}

type MockConfigRepo struct {
	mock.Mock
}

func (m *MockConfigRepo) Get(ctx context.Context) (*model.Config, error) {
	args := m.Called(ctx)
	return args.Get(0).(*model.Config), args.Error(1)
}

func (m *MockConfigRepo) Save(ctx context.Context, config *model.Config) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func TestBridgeService_GetDevices(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)

	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetDevices", mock.Anything).Return([]*model.Device{
		{ID: "1", Name: "Test Light", Type: model.DeviceTypeLight, State: &huego.State{On: true}},
	}, nil)

	s := NewBridgeService(mockHA, mockRepo)
	devices, err := s.GetDevices(context.Background())

	assert.NoError(t, err)
	assert.Len(t, devices, 1)
	assert.Equal(t, "Test Light", devices[0].Name)
}

func TestBridgeService_UpdateDeviceState(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	dev := &model.Device{ID: "1", Name: "Test Light", Type: model.DeviceTypeLight, State: &huego.State{On: false}}

	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetDevices", mock.Anything).Return([]*model.Device{dev}, nil)
	mockHA.On("SetState", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	s := NewBridgeService(mockHA, mockRepo)
	_, _ = s.GetDevices(context.Background()) // Load devices

	err := s.UpdateDeviceState(context.Background(), "1", map[string]interface{}{"on": true, "bri": float64(254)})
	assert.NoError(t, err)

	assert.True(t, dev.State.On)
	assert.Equal(t, uint8(254), dev.State.Bri)

	time.Sleep(100 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_GetDevice(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	dev := &model.Device{ID: "1", Name: "Test Light", Type: model.DeviceTypeLight, State: &huego.State{On: true}}

	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetDevices", mock.Anything).Return([]*model.Device{dev}, nil)

	s := NewBridgeService(mockHA, mockRepo)
	_, _ = s.GetDevices(context.Background())

	d, err := s.GetDevice(context.Background(), "1")
	assert.NoError(t, err)
	assert.Equal(t, dev, d)

	_, err = s.GetDevice(context.Background(), "2")
	assert.Error(t, err)
}
