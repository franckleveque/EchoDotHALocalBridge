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
		return fmt.Errorf("Home Assistant is not configured")
	}
	devices, err := s.haPort.GetDevices(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	newDevices := make(map[string]*model.Device)
	for _, d := range devices {
		newDevices[d.ID] = d
	}
	s.devices = newDevices
	return nil
}

func (s *BridgeService) GetDevices(ctx context.Context) ([]*model.Device, error) {
	if !s.haPort.IsConfigured() {
		return []*model.Device{}, nil
	}
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
		devices = append(devices, d)
	}
	return devices, nil
}

func (s *BridgeService) GetDevice(ctx context.Context, id string) (*model.Device, error) {
	s.mu.RLock()
	d, ok := s.devices[id]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("device %s not found", id)
	}
	return d, nil
}

func (s *BridgeService) UpdateDeviceState(ctx context.Context, id string, hueStateUpdate map[string]interface{}) error {
	s.mu.RLock()
	device, ok := s.devices[id]
	s.mu.RUnlock()
	if !ok {
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

	params := t.ToHA(tmpState)

	go func() {
		err := s.haPort.SetState(context.Background(), device, params)
		if err != nil {
			fmt.Printf("Error setting HA state: %v\n", err)
		}
	}()

	if on, ok := hueStateUpdate["on"].(bool); ok {
		device.State.On = on
	}
	if bri, ok := hueStateUpdate["bri"].(float64); ok {
		device.State.Bri = uint8(bri)
	}

	return nil
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
