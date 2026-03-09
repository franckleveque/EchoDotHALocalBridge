package service

import (
	"context"
	"fmt"
	"log/slog"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"
	"sort"
	"strconv"
	"sync"
	"time"
)

var RefreshInterval = 30 * time.Second

type BridgeService struct {
	haPort            ports.ReconfigurableHomeAssistantPort
	configRepo        ports.ConfigRepository
	translatorFactory ports.TranslatorFactory
	devices           map[string]*model.Device
	mu                sync.RWMutex
	refreshMu         sync.Mutex
	lastRefresh       time.Time
	initialized       bool
	cachedHAStates    []model.HAEntityState
	ignoredDomains    []string
}

func NewBridgeService(haPort ports.ReconfigurableHomeAssistantPort, configRepo ports.ConfigRepository, translatorFactory ports.TranslatorFactory) *BridgeService {
	s := &BridgeService{
		haPort:            haPort,
		configRepo:        configRepo,
		translatorFactory: translatorFactory,
		devices:           make(map[string]*model.Device),
	}
	return s
}

func (s *BridgeService) Start(ctx context.Context) {
	ticker := time.NewTicker(RefreshInterval)
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

func (s *BridgeService) TestDeviceAction(ctx context.Context, vd *model.VirtualDevice, state *model.DeviceState) error {
	// Create a dummy device for SetState
	dummyDevice := &model.Device{
		ID:            "test",
		Name:          vd.Name,
		Type:          vd.Type,
		ExternalID:    vd.EntityID,
		State:         state,
		VirtualDevice: vd,
	}

	t := s.translatorFactory.GetTranslator(vd.Type)
	cmd := t.ToHA(state, vd)

	go func() {
		err := s.haPort.SetState(context.Background(), dummyDevice, cmd)
		if err != nil {
			slog.Error("Error setting HA test state", "error", err)
		}
	}()

	return nil
}

func (s *BridgeService) RefreshDevices(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if time.Since(s.lastRefresh) < 2*time.Second && s.initialized {
		return nil
	}

	slog.Info("Bridge: refreshing devices from HA")

	cfg, err := s.configRepo.Get(ctx)
	if err != nil {
		return err
	}

	states, err := s.haPort.GetRawStates(ctx)
	if err != nil {
		slog.Error("Bridge: error getting HA states", "error", err)
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedHAStates = states

	// Index HA states by entity_id
	stateMap := make(map[string]model.HAEntityState)
	for _, state := range states {
		stateMap[state.EntityID] = state
	}

	newDevices := make(map[string]*model.Device)

	for _, vd := range cfg.VirtualDevices {
		state, exists := stateMap[vd.EntityID]
		if !exists {
			state = model.HAEntityState{EntityID: vd.EntityID, State: "unavailable"}
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
	slog.Info("Bridge: refreshed devices", "count", len(s.devices))
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

func (s *BridgeService) GetDeviceMetadata(deviceType model.MappingType) model.HueMetadata {
	return s.translatorFactory.GetTranslator(deviceType).GetMetadata()
}

func (s *BridgeService) UpdateDeviceState(ctx context.Context, id string, stateUpdate *model.DeviceState) error {
	s.mu.Lock()
	device, ok := s.devices[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("device %s not found", id)
	}

	// Create a temporary state merged with the current state to handle partial updates
	tmpState := *device.State
	if stateUpdate.UpdatedByBri {
		tmpState.Bri = stateUpdate.Bri
		tmpState.UpdatedByBri = true
		tmpState.On = true
	} else {
		tmpState.On = stateUpdate.On
	}

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
	if stateUpdate.UpdatedByBri {
		device.State.Bri = stateUpdate.Bri
		device.State.UpdatedByBri = true
		device.State.On = true
	} else {
		device.State.On = stateUpdate.On
	}

	deviceCopy := s.copyDevice(device)
	t := s.translatorFactory.GetTranslator(device.Type)
	cmd := t.ToHA(&tmpState, device.VirtualDevice)
	s.mu.Unlock()

	go func() {
		err := s.haPort.SetState(context.Background(), deviceCopy, cmd)
		if err != nil {
			slog.Error("Error setting HA state", "error", err)
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
	s.assignHueIDs(cfg)

	err := s.configRepo.Save(ctx, cfg)
	if err != nil {
		return err
	}
	s.haPort.Configure(cfg.HassURL, cfg.HassToken)

	// Force refresh
	s.refreshMu.Lock()
	s.lastRefresh = time.Now().Add(-5 * time.Second) // Ensure we can refresh
	s.refreshMu.Unlock()

	// We don't want to fail the whole update if Home Assistant is currently unreachable
	_ = s.RefreshDevices(ctx)

	return nil
}

func (s *BridgeService) GetAllEntities(ctx context.Context) ([]ports.HomeAssistantEntity, error) {
	s.mu.RLock()
	if s.initialized && time.Since(s.lastRefresh) < 2*time.Second {
		entities := s.extractEntities(s.cachedHAStates, s.ignoredDomains)
		s.mu.RUnlock()
		return entities, nil
	}
	s.mu.RUnlock()

	if err := s.RefreshDevices(ctx); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.extractEntities(s.cachedHAStates, s.ignoredDomains), nil
}

func (s *BridgeService) assignHueIDs(cfg *model.Config) {
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
}

func (s *BridgeService) SetIgnoredDomains(domains []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ignoredDomains = domains
}

func (s *BridgeService) extractEntities(states []model.HAEntityState, ignored []string) []ports.HomeAssistantEntity {
	var entities []ports.HomeAssistantEntity
	for _, s := range states {
		if !s.IsSupported(ignored) {
			continue
		}

		name := ""
		if s.Attributes != nil {
			if n, ok := s.Attributes["friendly_name"].(string); ok {
				name = n
			} else if n, ok := s.Attributes["name"].(string); ok {
				name = n
			}
		}
		if name == "" {
			name = s.EntityID
		}

		entities = append(entities, ports.HomeAssistantEntity{
			EntityID:     s.EntityID,
			FriendlyName: name,
		})
	}
	return entities
}

