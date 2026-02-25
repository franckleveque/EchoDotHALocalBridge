package service

import (
	"context"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"
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

func (m *MockHAPort) GetAllEntities(ctx context.Context) ([]ports.HomeAssistantEntity, error) {
	args := m.Called(ctx)
	return args.Get(0).([]ports.HomeAssistantEntity), args.Error(1)
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

func TestBridgeService_PayloadMerging(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	vd := &model.VirtualDevice{
		HueID:    "1",
		Name:     "Payload Test",
		EntityID: "camera.salon",
		Type:     model.MappingTypeCustom,
		ActionConfig: &model.ActionConfig{
			OnService: "camera.record",
			OnPayload: map[string]interface{}{"duration": 30.0},
		},
	}

	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{vd}}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}{
		{"entity_id": "camera.salon", "state": "idle"},
	}, nil)

	mockHA.On("SetState", mock.Anything, mock.Anything, mock.MatchedBy(func(p map[string]interface{}) bool {
		return p["service"] == "camera.record" && p["duration"] == 30.0
	})).Return(nil)

	s := NewBridgeService(mockHA, mockRepo)
	_, _ = s.GetDevices(context.Background())

	err := s.UpdateDeviceState(context.Background(), "1", map[string]interface{}{"on": true})
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_NoOpAndPayload(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	vd := &model.VirtualDevice{
		HueID:    "1",
		Name:     "NoOp Test",
		EntityID: "light.noop",
		Type:     model.MappingTypeLight,
		ActionConfig: &model.ActionConfig{
			NoOpOff: true,
		},
	}

	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{vd}}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}{
		{"entity_id": "light.noop", "state": "on"},
	}, nil)

	s := NewBridgeService(mockHA, mockRepo)
	_, _ = s.GetDevices(context.Background())

	// Update to OFF - should be NoOp (no call to SetState)
	err := s.UpdateDeviceState(context.Background(), "1", map[string]interface{}{"on": false})
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	mockHA.AssertExpectations(t) // Verify no SetState was called
}

func TestBridgeService_OmitEntityID(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	vd := &model.VirtualDevice{
		HueID:    "1",
		Name:     "Omit Test",
		EntityID: "script.test",
		Type:     model.MappingTypeCustom,
		ActionConfig: &model.ActionConfig{
			OmitEntityID: true,
		},
	}

	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{vd}}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}{}, nil)

	mockHA.On("SetState", mock.Anything, mock.MatchedBy(func(d *model.Device) bool {
		return d.ExternalID == "script.test"
	}), mock.MatchedBy(func(p map[string]interface{}) bool {
		_, exists := p["entity_id"]
		return !exists
	})).Return(nil)

	s := NewBridgeService(mockHA, mockRepo)
	_, _ = s.GetDevices(context.Background())

	_ = s.UpdateDeviceState(context.Background(), "1", map[string]interface{}{"on": true})

	time.Sleep(50 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_RefreshDevices_Error(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)

	mockRepo.On("Get", mock.Anything).Return(&model.Config{}, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}(nil), fmt.Errorf("api error"))

	s := NewBridgeService(mockHA, mockRepo)
	err := s.RefreshDevices(context.Background())
	assert.Error(t, err)
}

func TestBridgeService_GetDevices(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)

	cfg := &model.Config{
		VirtualDevices: []*model.VirtualDevice{
			{HueID: "1", Name: "Test Light", EntityID: "light.test", Type: model.MappingTypeLight},
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

	// Test initialized path
	devices2, err := s.GetDevices(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, devices[0].Name, devices2[0].Name)
}

func TestBridgeService_MultiIntentions(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)

	cfg := &model.Config{
		VirtualDevices: []*model.VirtualDevice{
			{HueID: "1", Name: "Detection Salon", EntityID: "camera.salon", Type: model.MappingTypeCustom},
			{HueID: "2", Name: "Clip Salon", EntityID: "camera.salon", Type: model.MappingTypeCustom},
		},
	}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}{
		{"entity_id": "camera.salon", "state": "idle"},
	}, nil)

	s := NewBridgeService(mockHA, mockRepo)
	devices, err := s.GetDevices(context.Background())

	assert.NoError(t, err)
	assert.Len(t, devices, 2)
	assert.Equal(t, "Detection Salon", devices[0].Name)
	assert.Equal(t, "Clip Salon", devices[1].Name)
}

func TestBridgeService_UpdateDeviceState(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	vd := &model.VirtualDevice{
		HueID:    "1",
		Name:     "Test Light",
		EntityID: "light.test",
		Type:     model.MappingTypeLight,
		ActionConfig: &model.ActionConfig{
			OnPayload: map[string]interface{}{"attr": "val"},
		},
	}

	cfg := &model.Config{
		VirtualDevices: []*model.VirtualDevice{vd},
	}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]map[string]interface{}{
		{"entity_id": "light.test", "state": "off"},
	}, nil)

	mockHA.On("SetState", mock.Anything, mock.Anything, mock.MatchedBy(func(p map[string]interface{}) bool {
		return p["service"] == "turn_on"
	})).Return(nil)

	s := NewBridgeService(mockHA, mockRepo)
	_, _ = s.GetDevices(context.Background()) // Load devices

	// Update to ON
	err := s.UpdateDeviceState(context.Background(), "1", map[string]interface{}{"on": true})
	assert.NoError(t, err)

	d, _ := s.GetDevice(context.Background(), "1")
	assert.True(t, d.State.On)

	time.Sleep(100 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_GetAllEntities(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	entities := []ports.HomeAssistantEntity{{EntityID: "light.test", FriendlyName: "Test Light"}}
	mockHA.On("GetAllEntities", mock.Anything).Return(entities, nil)

	s := NewBridgeService(mockHA, mockRepo)
	res, err := s.GetAllEntities(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, entities, res)
}

func TestBridgeService_Config(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	cfg := &model.Config{
		HassURL:   "http://localhost",
		HassToken: "token",
		VirtualDevices: []*model.VirtualDevice{
			{Name: "New Device", EntityID: "light.new", Type: model.MappingTypeLight},
		},
	}

	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockRepo.On("Save", mock.Anything, mock.MatchedBy(func(c *model.Config) bool {
		return c.VirtualDevices[0].HueID == "1"
	})).Return(nil)
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

func TestBridgeService_RefreshCooldown(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)

	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{}}
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
