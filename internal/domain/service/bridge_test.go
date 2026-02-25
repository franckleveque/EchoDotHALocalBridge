package service

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockHAPort struct {
	mock.Mock
}

func (m *MockHAPort) GetRawStates(ctx context.Context) ([]map[string]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (m *MockHAPort) GetAllEntities(ctx context.Context) ([]*model.EntityMapping, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*model.EntityMapping), args.Error(1)
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

	cfg := &model.Config{
		EntityMappings: map[string]*model.EntityMapping{
			"light.test": {EntityID: "light.test", HueID: "1", Name: "Test Light", Type: model.MappingTypeLight, Exposed: true},
		},
	}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}{
		{"entity_id": "light.test", "state": "on", "attributes": map[string]interface{}{"brightness": 100.0}},
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
	mapping := &model.EntityMapping{EntityID: "light.test", HueID: "1", Name: "Test Light", Type: model.MappingTypeLight, Exposed: true}

	cfg := &model.Config{
		EntityMappings: map[string]*model.EntityMapping{"light.test": mapping},
	}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}{
		{"entity_id": "light.test", "state": "off"},
	}, nil)
	mockHA.On("SetState", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	s := NewBridgeService(mockHA, mockRepo)
	_, _ = s.GetDevices(context.Background()) // Load devices

	// Partial update: just bri. It should remain "on" if it was already "on" or keep its state.
	// But our mock devices starts as "off" in this test.
	err := s.UpdateDeviceState(context.Background(), "1", map[string]interface{}{"bri": float64(254)})
	assert.NoError(t, err)

	// Check optimistic update on the device in memory (cached in service)
	d, _ := s.GetDevice(context.Background(), "1")
	assert.Equal(t, uint8(254), d.State.Bri)
	// It should still be OFF because it was off and we didn't send "on": true
	assert.False(t, d.State.On)

	time.Sleep(100 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_GetAllEntities(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	entities := []*model.EntityMapping{{EntityID: "light.test"}}
	mockHA.On("GetAllEntities", mock.Anything).Return(entities, nil)

	s := NewBridgeService(mockHA, mockRepo)
	res, err := s.GetAllEntities(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, entities, res)
}

func TestBridgeService_GetDevices_Empty(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)

	mockHA.On("IsConfigured").Return(false)

	s := NewBridgeService(mockHA, mockRepo)
	devices, err := s.GetDevices(context.Background())

	assert.NoError(t, err)
	assert.Len(t, devices, 0)
}

func TestBridgeService_Config(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	cfg := &model.Config{HassURL: "http://localhost", HassToken: "token"}

	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockRepo.On("Save", mock.Anything, cfg).Return(nil)
	mockHA.On("Configure", "http://localhost", "token").Return()
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}{}, nil)

	s := NewBridgeService(mockHA, mockRepo)

	// Test GetConfig
	res, err := s.GetConfig(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, cfg, res)

	// Test UpdateConfig
	err = s.UpdateConfig(context.Background(), cfg)
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_UpdateConfigStableIDs(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)

	cfg := &model.Config{
		EntityMappings: map[string]*model.EntityMapping{
			"light.1": {EntityID: "light.1", HueID: "10"},
			"light.2": {EntityID: "light.2", HueID: ""},
		},
	}

	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockRepo.On("Save", mock.Anything, mock.MatchedBy(func(c *model.Config) bool {
		return c.EntityMappings["light.2"].HueID == "11"
	})).Return(nil)
	mockHA.On("Configure", mock.Anything, mock.Anything).Return()
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}{}, nil)

	s := NewBridgeService(mockHA, mockRepo)
	err := s.UpdateConfig(context.Background(), cfg)
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestBridgeService_RefreshCooldown(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)

	cfg := &model.Config{EntityMappings: make(map[string]*model.EntityMapping)}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}{}, nil).Once()

	s := NewBridgeService(mockHA, mockRepo)

	// First call
	err := s.RefreshDevices(context.Background())
	assert.NoError(t, err)

	// Second call immediately - should skip (mock only expects one call to GetRawStates)
	err = s.RefreshDevices(context.Background())
	assert.NoError(t, err)

	mockHA.AssertExpectations(t)
}
