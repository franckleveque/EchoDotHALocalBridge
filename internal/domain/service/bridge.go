package service

import (
	"context"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/domain/translator"
	"hue-bridge-emulator/internal/ports"
	"sync"
	"github.com/amimof/huego"
)

type BridgeService struct {
	haPort            ports.HomeAssistantPort
	configRepo        ports.ConfigRepository
	translatorFactory *translator.Factory
	devices           map[string]*model.Device
	mu                sync.RWMutex
}

func NewBridgeService(haPort ports.HomeAssistantPort, configRepo ports.ConfigRepository) *BridgeService {
	return &BridgeService{
		haPort:            haPort,
		configRepo:        configRepo,
		translatorFactory: translator.NewFactory(),
		devices:           make(map[string]*model.Device),
	}
}

func (s *BridgeService) RefreshDevices(ctx context.Context) error {
	if !s.haPort.IsConfigured() {
		return nil
	}

	cfg, err := s.configRepo.Get(ctx)
	if err != nil {
		return err
	}

	states, err := s.haPort.GetRawStates(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	newDevices := make(map[string]*model.Device)

	for _, state := range states {
		entityID, _ := state["entity_id"].(string)
		mapping, ok := cfg.EntityMappings[entityID]
		if !ok || !mapping.Exposed {
			continue
		}

		t := s.translatorFactory.GetTranslator(mapping.Type)
		hueState := t.ToHue(state, mapping)

		newDevices[mapping.HueID] = &model.Device{
			ID:         mapping.HueID,
			Name:       mapping.Name,
			Type:       mapping.Type,
			ExternalID: entityID,
			State:      hueState,
			Mapping:    mapping,
		}
	}
	s.devices = newDevices
	return nil
}

func (s *BridgeService) GetDevices(ctx context.Context) ([]*model.Device, error) {
	s.mu.RLock()
	if len(s.devices) == 0 {
		s.mu.RUnlock()
		err := s.RefreshDevices(ctx)
		if err != nil {
			return nil, err
		}
		s.mu.RLock()
	}
	defer s.mu.RUnlock()
	devices := make([]*model.Device, 0, len(s.devices))
	for _, d := range s.devices {
		devices = append(devices, s.copyDevice(d))
	}
	return devices, nil
}

func (s *BridgeService) GetDevice(ctx context.Context, id string) (*model.Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.devices[id]
	if !ok {
		return nil, fmt.Errorf("device %s not found", id)
	}
	return s.copyDevice(d), nil
}

func (s *BridgeService) UpdateDeviceState(ctx context.Context, id string, hueStateUpdate map[string]interface{}) error {
	s.mu.Lock()
	device, ok := s.devices[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("device %s not found", id)
	}

	t := s.translatorFactory.GetTranslator(device.Type)

	tmpState := &huego.State{}
	if on, ok := hueStateUpdate["on"].(bool); ok {
		tmpState.On = on
	}
	if bri, ok := hueStateUpdate["bri"].(float64); ok {
		tmpState.Bri = uint8(bri)
	}

	params := t.ToHA(tmpState, device.Mapping)

	// Optimistic update under lock
	if on, ok := hueStateUpdate["on"].(bool); ok {
		device.State.On = on
	}
	if bri, ok := hueStateUpdate["bri"].(float64); ok {
		device.State.Bri = uint8(bri)
	}

	deviceCopy := s.copyDevice(device)
	s.mu.Unlock()

	go func() {
		err := s.haPort.SetState(context.Background(), deviceCopy, params)
		if err != nil {
			fmt.Printf("Error setting HA state: %v\n", err)
		}
	}()

	return nil
}

func (s *BridgeService) copyDevice(d *model.Device) *model.Device {
	dCopy := *d
	if d.State != nil {
		stateCopy := *d.State
		if d.State.Xy != nil {
			stateCopy.Xy = make([]float32, len(d.State.Xy))
			copy(stateCopy.Xy, d.State.Xy)
		}
		dCopy.State = &stateCopy
	}
	return &dCopy
}

func (s *BridgeService) GetConfig(ctx context.Context) (*model.Config, error) {
	return s.configRepo.Get(ctx)
}

func (s *BridgeService) UpdateConfig(ctx context.Context, cfg *model.Config) error {
	err := s.configRepo.Save(ctx, cfg)
	if err != nil {
		return err
	}
	s.haPort.Configure(cfg.HassURL, cfg.HassToken)
	return s.RefreshDevices(ctx)
}

func (s *BridgeService) GetAllEntities(ctx context.Context) ([]*model.EntityMapping, error) {
	return s.haPort.GetAllEntities(ctx)
}
