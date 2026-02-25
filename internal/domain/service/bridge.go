package service

import (
	"context"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/domain/translator"
	"hue-bridge-emulator/internal/ports"
	"sort"
	"strconv"
	"sync"
	"time"
)

type BridgeService struct {
	haPort            ports.HomeAssistantPort
	configRepo        ports.ConfigRepository
	translatorFactory *translator.Factory
	devices           map[string]*model.Device
	mu                sync.RWMutex
	refreshMu         sync.Mutex
	lastRefresh       time.Time
	initialized       bool
}

func NewBridgeService(haPort ports.HomeAssistantPort, configRepo ports.ConfigRepository) *BridgeService {
	s := &BridgeService{
		haPort:            haPort,
		configRepo:        configRepo,
		translatorFactory: translator.NewFactory(),
		devices:           make(map[string]*model.Device),
	}
	return s
}

func (s *BridgeService) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.RefreshDevices(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *BridgeService) RefreshDevices(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if time.Since(s.lastRefresh) < 2*time.Second && s.initialized {
		return nil
	}

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

	// Index HA states by entity_id
	stateMap := make(map[string]map[string]interface{})
	for _, state := range states {
		if eid, ok := state["entity_id"].(string); ok {
			stateMap[eid] = state
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	newDevices := make(map[string]*model.Device)

	for _, vd := range cfg.VirtualDevices {
		state := stateMap[vd.EntityID]
		if state == nil {
			state = map[string]interface{}{"entity_id": vd.EntityID, "state": "unavailable"}
		}

		t := s.translatorFactory.GetTranslator(vd.Type)
		hueState := t.ToHue(state, vd)

		newDevices[vd.HueID] = &model.Device{
			ID:            vd.HueID,
			Name:          vd.Name,
			Type:          vd.Type,
			ExternalID:    vd.EntityID,
			State:         hueState,
			VirtualDevice: vd,
		}
	}
	s.devices = newDevices
	s.lastRefresh = time.Now()
	s.initialized = true
	return nil
}

func (s *BridgeService) GetDevices(ctx context.Context) ([]*model.Device, error) {
	s.mu.RLock()
	if s.initialized {
		devices := s.getDevicesLocked()
		s.mu.RUnlock()
		return devices, nil
	}
	s.mu.RUnlock()

	if err := s.RefreshDevices(ctx); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getDevicesLocked(), nil
}

// getDevicesLocked returns the devices list, must be called with at least a read lock
func (s *BridgeService) getDevicesLocked() []*model.Device {
	devices := make([]*model.Device, 0, len(s.devices))
	for _, d := range s.devices {
		devices = append(devices, s.copyDevice(d))
	}

	// Stable sorting by HueID (numeric)
	sort.Slice(devices, func(i, j int) bool {
		idI, _ := strconv.Atoi(devices[i].ID)
		idJ, _ := strconv.Atoi(devices[j].ID)
		return idI < idJ
	})
	return devices
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

	// Create a temporary state merged with the current state to handle partial updates
	tmpState := *device.State
	if on, ok := hueStateUpdate["on"].(bool); ok {
		tmpState.On = on
	}
	if bri, ok := hueStateUpdate["bri"].(float64); ok {
		tmpState.Bri = uint8(bri)
	}

	serviceName, params := t.ToHA(&tmpState, device.VirtualDevice)
	params["service"] = serviceName

	// Check NoOp before starting goroutine
	vd := device.VirtualDevice
	if vd.ActionConfig != nil {
		if tmpState.On && vd.ActionConfig.NoOpOn {
			s.mu.Unlock()
			return nil
		}
		if !tmpState.On && vd.ActionConfig.NoOpOff {
			s.mu.Unlock()
			return nil
		}
	}

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
	// Ensure stable Hue IDs
	maxID := 0
	for _, vd := range cfg.VirtualDevices {
		if vd.HueID != "" {
			if id, err := strconv.Atoi(vd.HueID); err == nil && id > maxID {
				maxID = id
			}
		}
	}

	for _, vd := range cfg.VirtualDevices {
		if vd.HueID == "" {
			maxID++
			vd.HueID = strconv.Itoa(maxID)
		}
	}

	err := s.configRepo.Save(ctx, cfg)
	if err != nil {
		return err
	}
	s.haPort.Configure(cfg.HassURL, cfg.HassToken)

	// Force refresh
	s.refreshMu.Lock()
	s.lastRefresh = time.Time{}
	s.refreshMu.Unlock()

	return s.RefreshDevices(ctx)
}

func (s *BridgeService) GetAllEntities(ctx context.Context) ([]ports.HomeAssistantEntity, error) {
	return s.haPort.GetAllEntities(ctx)
}
