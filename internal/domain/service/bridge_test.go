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

func (m *MockHAPort) GetRawStates(ctx context.Context) ([]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockHAPort) GetAllEntities(ctx context.Context) ([]ports.HomeAssistantEntity, error) {
	args := m.Called(ctx)
	return args.Get(0).([]ports.HomeAssistantEntity), args.Error(1)
}

func (m *MockHAPort) SetState(ctx context.Context, device *model.Device, cmd model.HomeAssistantCommand) error {
	args := m.Called(ctx, device, cmd)
	return args.Error(0)
}

func (m *MockHAPort) Configure(url, token string) {
	m.Called(url, token)
}

func (m *MockHAPort) IsConfigured() bool {
	args := m.Called()
	return args.Bool(0)
}

type MockTranslatorFactory struct {
	mock.Mock
}

func (m *MockTranslatorFactory) GetTranslator(mappingType model.MappingType) ports.Translator {
	args := m.Called(mappingType)
	return args.Get(0).(ports.Translator)
}

type MockTranslator struct {
	mock.Mock
}

func (m *MockTranslator) ToHue(haState any, vd *model.VirtualDevice) *model.DeviceState {
	args := m.Called(haState, vd)
	return args.Get(0).(*model.DeviceState)
}

func (m *MockTranslator) ToHA(hueState *model.DeviceState, vd *model.VirtualDevice) model.HomeAssistantCommand {
	args := m.Called(hueState, vd)
	return args.Get(0).(model.HomeAssistantCommand)
}

func (m *MockTranslator) GetMetadata() model.HueMetadata {
	args := m.Called()
	return args.Get(0).(model.HueMetadata)
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
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

	vd := &model.VirtualDevice{
		HueID:    "1",
		Name:     "Payload Test",
		EntityID: "camera.salon",
		Type:     model.MappingTypeCustom,
	}

	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{vd}}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{
		map[string]interface{}{"entity_id": "camera.salon", "state": "idle"},
	}, nil)

	mockTF.On("GetTranslator", model.MappingTypeCustom).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{On: false})
	mockT.On("ToHA", mock.Anything, mock.Anything).Return(model.HomeAssistantCommand{
		Service: "camera.record",
		Data:    map[string]interface{}{"duration": 30.0},
	})

	mockHA.On("SetState", mock.Anything, mock.Anything, mock.MatchedBy(func(cmd model.HomeAssistantCommand) bool {
		p := cmd.Data.(map[string]interface{})
		return cmd.Service == "camera.record" && p["duration"] == 30.0
	})).Return(nil)

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	_, _ = s.GetDevices(context.Background())

	err := s.UpdateDeviceState(context.Background(), "1", &model.DeviceState{On: true})
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_NoOpAndPayload(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

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
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{
		map[string]interface{}{"entity_id": "light.noop", "state": "on"},
	}, nil)

	mockTF.On("GetTranslator", model.MappingTypeLight).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{On: true})

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	_, _ = s.GetDevices(context.Background())

	// Update to OFF - should be NoOp (no call to SetState)
	err := s.UpdateDeviceState(context.Background(), "1", &model.DeviceState{On: false})
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	mockHA.AssertExpectations(t) // Verify no SetState was called
}

func TestBridgeService_OmitEntityID(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

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
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{}, nil)

	mockTF.On("GetTranslator", model.MappingTypeCustom).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{On: false})
	mockT.On("ToHA", mock.Anything, mock.Anything).Return(model.HomeAssistantCommand{
		Service: "script.test",
		Data:    map[string]interface{}{},
	})

	mockHA.On("SetState", mock.Anything, mock.MatchedBy(func(d *model.Device) bool {
		return d.ExternalID == "script.test"
	}), mock.Anything).Return(nil)

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	_, _ = s.GetDevices(context.Background())

	_ = s.UpdateDeviceState(context.Background(), "1", &model.DeviceState{On: true})

	time.Sleep(50 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_RefreshDevices_Error(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)

	// Error getting config
	mockRepo.On("Get", mock.Anything).Return((*model.Config)(nil), fmt.Errorf("config error")).Once()
	mockHA.On("IsConfigured").Return(true)
	s := NewBridgeService(mockHA, mockRepo, mockTF)
	err := s.RefreshDevices(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "config error", err.Error())

	// Error getting states
	mockRepo.On("Get", mock.Anything).Return(&model.Config{}, nil).Once()
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}(nil), fmt.Errorf("api error")).Once()
	// Set lastRefresh to old value to bypass cooldown
	s.lastRefresh = time.Now().Add(-10 * time.Second)

	err = s.RefreshDevices(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "api error", err.Error())

	// Coverage for invalid raw state
	mockRepo.On("Get", mock.Anything).Return(&model.Config{}, nil).Once()
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{"invalid"}, nil).Once()
	// Set lastRefresh to old value to bypass cooldown
	s.lastRefresh = time.Now().Add(-10 * time.Second)
	err = s.RefreshDevices(context.Background())
	assert.NoError(t, err)
}

func TestBridgeService_GetDevices(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

	cfg := &model.Config{
		VirtualDevices: []*model.VirtualDevice{
			{HueID: "1", Name: "Test Light", EntityID: "light.test", Type: model.MappingTypeLight},
		},
	}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{
		map[string]interface{}{"entity_id": "light.test", "state": "on", "attributes": map[string]interface{}{"brightness": 100.0}},
	}, nil)

	mockTF.On("GetTranslator", model.MappingTypeLight).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{On: true, Bri: 100})

	s := NewBridgeService(mockHA, mockRepo, mockTF)
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
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

	cfg := &model.Config{
		VirtualDevices: []*model.VirtualDevice{
			{HueID: "1", Name: "Detection Salon", EntityID: "camera.salon", Type: model.MappingTypeCustom},
			{HueID: "2", Name: "Clip Salon", EntityID: "camera.salon", Type: model.MappingTypeCustom},
		},
	}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{
		map[string]interface{}{"entity_id": "camera.salon", "state": "idle"},
	}, nil)

	mockTF.On("GetTranslator", model.MappingTypeCustom).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{On: false})

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	devices, err := s.GetDevices(context.Background())

	assert.NoError(t, err)
	assert.Len(t, devices, 2)
	assert.Equal(t, "Detection Salon", devices[0].Name)
	assert.Equal(t, "Clip Salon", devices[1].Name)
}

func TestBridgeService_UpdateDeviceState(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

	vd := &model.VirtualDevice{
		HueID:    "1",
		Name:     "Test Light",
		EntityID: "light.test",
		Type:     model.MappingTypeLight,
	}

	cfg := &model.Config{
		VirtualDevices: []*model.VirtualDevice{vd},
	}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{
		map[string]interface{}{"entity_id": "light.test", "state": "off"},
	}, nil)

	mockTF.On("GetTranslator", model.MappingTypeLight).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{On: false})
	mockT.On("ToHA", mock.Anything, mock.Anything).Return(model.HomeAssistantCommand{Service: "turn_on"})

	mockHA.On("SetState", mock.Anything, mock.Anything, mock.MatchedBy(func(cmd model.HomeAssistantCommand) bool {
		return cmd.Service == "turn_on"
	})).Return(nil)

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	_, _ = s.GetDevices(context.Background()) // Load devices

	// Update to ON
	err := s.UpdateDeviceState(context.Background(), "1", &model.DeviceState{On: true})
	assert.NoError(t, err)

	d, _ := s.GetDevice(context.Background(), "1")
	assert.True(t, d.State.On)

	time.Sleep(100 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_GetAllEntities(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	entities := []ports.HomeAssistantEntity{{EntityID: "light.test", FriendlyName: "Test Light"}}
	mockHA.On("GetAllEntities", mock.Anything).Return(entities, nil)

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	res, err := s.GetAllEntities(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, entities, res)
}

func TestBridgeService_Config(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

	cfg := &model.Config{
		HassURL:   "http://localhost",
		HassToken: "token",
		VirtualDevices: []*model.VirtualDevice{
			{Name: "New Device", EntityID: "light.new", Type: model.MappingTypeLight, HueID: "5"},
			{Name: "Another", EntityID: "light.another", Type: model.MappingTypeLight},
		},
	}

	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockRepo.On("Save", mock.Anything, mock.MatchedBy(func(c *model.Config) bool {
		// Verify HueID assignment: 5 was max, so next should be 6
		return c.VirtualDevices[1].HueID == "6"
	})).Return(nil)
	mockHA.On("Configure", "http://localhost", "token").Return()
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{}, nil)

	mockTF.On("GetTranslator", mock.Anything).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{})

	s := NewBridgeService(mockHA, mockRepo, mockTF)

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

func TestBridgeService_UpdateConfig_Errors(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{
		{HueID: "invalid"},
	}}
	mockRepo.On("Save", mock.Anything, mock.Anything).Return(fmt.Errorf("save error")).Once()

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	err := s.UpdateConfig(context.Background(), cfg)
	assert.Error(t, err)
}

func TestBridgeService_GetDevices_Error(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockHA.On("IsConfigured").Return(true)
	mockRepo.On("Get", mock.Anything).Return(&model.Config{}, fmt.Errorf("repo error"))

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	_, err := s.GetDevices(context.Background())
	assert.Error(t, err)
}

func TestBridgeService_UpdateDeviceState_SetStateError(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

	vd := &model.VirtualDevice{HueID: "1", EntityID: "light.test", Type: model.MappingTypeLight}
	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{vd}}

	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{map[string]interface{}{"entity_id": "light.test", "state": "on"}}, nil)

	mockTF.On("GetTranslator", model.MappingTypeLight).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{On: true})
	mockT.On("ToHA", mock.Anything, mock.Anything).Return(model.HomeAssistantCommand{Service: "turn_off"})

	mockHA.On("SetState", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("HA error")).Once()

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	_, _ = s.GetDevices(context.Background())

	err := s.UpdateDeviceState(context.Background(), "1", &model.DeviceState{On: false})
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_TestDeviceAction(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

	vd := &model.VirtualDevice{
		Name:     "Test Light",
		EntityID: "light.test",
		Type:     model.MappingTypeLight,
	}

	mockTF.On("GetTranslator", model.MappingTypeLight).Return(mockT)
	mockT.On("ToHA", mock.Anything, mock.Anything).Return(model.HomeAssistantCommand{Service: "turn_on"}).Times(3)

	// Test case 1: Turn ON
	mockHA.On("SetState", mock.Anything, mock.MatchedBy(func(d *model.Device) bool {
		return d.Name == "Test Light" && d.ExternalID == "light.test"
	}), mock.MatchedBy(func(cmd model.HomeAssistantCommand) bool {
		return cmd.Service == "turn_on"
	})).Return(nil).Once()

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	err := s.TestDeviceAction(context.Background(), vd, &model.DeviceState{On: true})
	assert.NoError(t, err)

	// Test case 2: Bri update without explicit ON
	mockHA.On("SetState", mock.Anything, mock.MatchedBy(func(d *model.Device) bool {
		return d.ExternalID == "light.test"
	}), mock.MatchedBy(func(cmd model.HomeAssistantCommand) bool {
		return cmd.Service == "turn_on"
	})).Return(nil).Once()

	err = s.TestDeviceAction(context.Background(), vd, &model.DeviceState{Bri: 200, UpdatedByBri: true})
	assert.NoError(t, err)

	// Test case 3: Error in SetState
	mockHA.On("SetState", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("HA error")).Once()
	err = s.TestDeviceAction(context.Background(), vd, &model.DeviceState{On: false})
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_GetRawStates(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	states := []interface{}{map[string]interface{}{"entity_id": "light.test"}}
	mockHA.On("GetRawStates", mock.Anything).Return(states, nil)

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	res, err := s.haPort.GetRawStates(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, states, res)
}

func TestBridgeService_RefreshCooldown(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)

	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{}}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{}, nil).Once()

	s := NewBridgeService(mockHA, mockRepo, mockTF)

	// First call
	err := s.RefreshDevices(context.Background())
	assert.NoError(t, err)

	// Second call immediately - should skip (mock only expects one call to GetRawStates)
	err = s.RefreshDevices(context.Background())
	assert.NoError(t, err)

	mockHA.AssertExpectations(t)
}

func TestBridgeService_Refresh_NotConfigured(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockHA.On("IsConfigured").Return(false)
	s := NewBridgeService(mockHA, mockRepo, mockTF)
	err := s.RefreshDevices(context.Background())
	assert.NoError(t, err)
}

func TestBridgeService_Refresh_EntityNotFound(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)
	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{{HueID: "1", EntityID: "light.missing", Type: model.MappingTypeLight}}}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{}, nil)

	mockTF.On("GetTranslator", model.MappingTypeLight).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{})

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	err := s.RefreshDevices(context.Background())
	assert.NoError(t, err)
	d, _ := s.GetDevice(context.Background(), "1")
	assert.NotNil(t, d)
}

func TestBridgeService_Start(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)

	// Reduce interval for test
	oldInterval := RefreshInterval
	RefreshInterval = 10 * time.Millisecond
	defer func() { RefreshInterval = oldInterval }()

	mockHA.On("IsConfigured").Return(false) // Just to make RefreshDevices do nothing quickly

	ctx, cancel := context.WithCancel(context.Background())
	s := NewBridgeService(mockHA, mockRepo, mockTF)
	s.Start(ctx)

	time.Sleep(25 * time.Millisecond) // Should trigger at least one tick
	cancel()
	time.Sleep(10 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_GetDevice_NotFound(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	s := NewBridgeService(mockHA, mockRepo, mockTF)
	_, err := s.GetDevice(context.Background(), "99")
	assert.Error(t, err)
}

func TestBridgeService_UpdateDeviceState_NotFound(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	err := s.UpdateDeviceState(context.Background(), "99", &model.DeviceState{On: true})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBridgeService_UpdateDeviceState_NoOpOn(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

	vd := &model.VirtualDevice{
		HueID:    "1",
		Name:     "NoOp On Test",
		EntityID: "light.noop_on",
		Type:     model.MappingTypeLight,
		ActionConfig: &model.ActionConfig{
			NoOpOn: true,
		},
	}

	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{vd}}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{
		map[string]interface{}{"entity_id": "light.noop_on", "state": "off"},
	}, nil)

	mockTF.On("GetTranslator", model.MappingTypeLight).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{On: false})

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	_, _ = s.GetDevices(context.Background())

	// Update to ON - should be NoOp (no call to SetState)
	err := s.UpdateDeviceState(context.Background(), "1", &model.DeviceState{On: true})
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_UpdateDeviceState_BriWithoutOn(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

	vd := &model.VirtualDevice{
		HueID:    "1",
		Name:     "Bri Auto On Test",
		EntityID: "light.bri_on",
		Type:     model.MappingTypeLight,
	}

	cfg := &model.Config{VirtualDevices: []*model.VirtualDevice{vd}}
	mockRepo.On("Get", mock.Anything).Return(cfg, nil)
	mockHA.On("IsConfigured").Return(true)
	mockHA.On("GetRawStates", mock.Anything).Return([]interface{}{
		map[string]interface{}{"entity_id": "light.bri_on", "state": "off"},
	}, nil)

	mockTF.On("GetTranslator", model.MappingTypeLight).Return(mockT)
	mockT.On("ToHue", mock.Anything, mock.Anything).Return(&model.DeviceState{On: false})
	mockT.On("ToHA", mock.Anything, mock.Anything).Return(model.HomeAssistantCommand{Service: "turn_on"})

	mockHA.On("SetState", mock.Anything, mock.Anything, mock.MatchedBy(func(cmd model.HomeAssistantCommand) bool {
		return cmd.Service == "turn_on"
	})).Return(nil)

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	_, _ = s.GetDevices(context.Background())

	// Update with bri only
	err := s.UpdateDeviceState(context.Background(), "1", &model.DeviceState{Bri: 127, UpdatedByBri: true})
	assert.NoError(t, err)

	d, _ := s.GetDevice(context.Background(), "1")
	assert.True(t, d.State.On)
	assert.Equal(t, uint8(127), d.State.Bri)
	assert.True(t, d.State.UpdatedByBri)

	time.Sleep(50 * time.Millisecond)
	mockHA.AssertExpectations(t)
}

func TestBridgeService_CopyDevice(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	d := &model.Device{
		ID:   "1",
		Name: "Test",
		State: &model.DeviceState{
			On: true,
			Xy: []float32{0.1, 0.2},
		},
	}

	// Internal copy call
	dCopy := s.copyDevice(d)

	assert.NotNil(t, dCopy)
	assert.NotSame(t, d, dCopy)
	assert.NotSame(t, d.State, dCopy.State)
	// Xy is a slice, NotSame expects pointers to the slice header or something if they are not pointers?
	// Actually assert.NotSame checks if pointers are different.
	if len(d.State.Xy) > 0 {
		assert.NotSame(t, &d.State.Xy[0], &dCopy.State.Xy[0])
	}
	assert.Equal(t, d.State.Xy, dCopy.State.Xy)

	// Test nil state case
	d2 := &model.Device{ID: "2"}
	dCopy2 := s.copyDevice(d2)
	assert.Nil(t, dCopy2.State)
}

func TestBridgeService_GetDeviceMetadata(t *testing.T) {
	mockHA := new(MockHAPort)
	mockRepo := new(MockConfigRepo)
	mockTF := new(MockTranslatorFactory)
	mockT := new(MockTranslator)

	mockTF.On("GetTranslator", model.MappingTypeLight).Return(mockT)
	mockT.On("GetMetadata").Return(model.HueMetadata{Type: "TestType"})

	s := NewBridgeService(mockHA, mockRepo, mockTF)
	meta := s.GetDeviceMetadata(model.MappingTypeLight)
	assert.Equal(t, "TestType", meta.Type)
}
