package service

import (
	"context"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// nameAttributes lists the HA attributes used as display name, by priority.
var nameAttributes = []string{"friendly_name", "name"}

var RefreshInterval = 30 * time.Second

type BridgeService struct {
	haPort            ports.ReconfigurableHomeAssistantPort
	configRepo        ports.ConfigRepository
	translatorFactory ports.TranslatorFactory
	devices           map[string]*model.Device
	sortedDevices     []*model.Device
	mu                sync.RWMutex
	lastRefresh       time.Time
	initialized       bool
	cachedHAStates    []model.HAEntityState
	ignoredDomains    []string
	refreshGroup      singleflight.Group
	workerSem         chan struct{}
	logger            *slog.Logger
}

func NewBridgeService(haPort ports.ReconfigurableHomeAssistantPort, configRepo ports.ConfigRepository, translatorFactory ports.TranslatorFactory, logger *slog.Logger) *BridgeService {
	s := &BridgeService{
		haPort:            haPort,
		configRepo:        configRepo,
		translatorFactory: translatorFactory,
		logger:            logger,
		devices:           make(map[string]*model.Device),
		workerSem:         make(chan struct{}, 10), // Limit to 10 concurrent HA service calls
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

// TestDeviceAction runs a one-off HA call synchronously (10 s timeout) and returns
// the HA port error unchanged, so callers can inspect it with errors.As.
func (s *BridgeService) TestDeviceAction(ctx context.Context, vd *model.VirtualDevice, state *model.DeviceState) error {
	if vd == nil || state == nil {
		return model.ErrInvalidTestRequest
	}

	// Work on a copy: the caller's state must not be modified.
	stateCopy := *state
	if stateCopy.UpdatedByBri {
		stateCopy.On = true
	}

	// Same No-Op rule as UpdateDeviceState. Never return nil here: the caller
	// would report success although nothing reached Home Assistant.
	if ac := vd.ActionConfig; ac != nil {
		if (stateCopy.On && ac.NoOpOn) || (!stateCopy.On && ac.NoOpOff) {
			return model.ErrNoOpAction
		}
	}

	select {
	case s.workerSem <- struct{}{}:
		defer func() { <-s.workerSem }()
	default:
		return model.ErrTooManyRequests
	}

	// Create a dummy device for SetState
	dummyDevice := &model.Device{
		ID:            "test",
		Name:          vd.Name,
		Type:          vd.Type,
		ExternalID:    vd.EntityID,
		State:         &stateCopy,
		VirtualDevice: vd,
	}

	cmd := s.translatorFactory.ToHA(vd.Type, &stateCopy, vd)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.haPort.SetState(ctx, dummyDevice, cmd)
}

func (s *BridgeService) RefreshDevices(ctx context.Context) error {
	_, err, _ := s.refreshGroup.Do("refresh", func() (interface{}, error) {
		s.mu.RLock()
		if time.Since(s.lastRefresh) < 2*time.Second && s.initialized {
			s.mu.RUnlock()
			return nil, nil
		}
		s.mu.RUnlock()

		s.logger.Info("Bridge: refreshing devices from HA")

		cfg, err := s.configRepo.Get(ctx)
		if err != nil {
			return nil, err
		}

		states, err := s.haPort.GetRawStates(ctx)
		if err != nil {
			s.logger.Error("Bridge: error getting HA states", "error", err)
			return nil, err
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

		s.logger.Debug("Bridge: processing virtual devices", "count", len(cfg.VirtualDevices), "ha_state_map_size", len(stateMap))
		for _, vd := range cfg.VirtualDevices {
			state, exists := stateMap[vd.EntityID]
			if !exists {
				s.logger.Warn("Bridge: entity not found in HA states", "entity_id", vd.EntityID)
				state = model.HAEntityState{EntityID: vd.EntityID, State: "unavailable"}
			}

			hueState := s.translatorFactory.ToHue(vd.Type, state, vd)

			newDevices[vd.HueID] = &model.Device{
				ID:            vd.HueID,
				Name:          vd.Name,
				Type:          vd.Type,
				ExternalID:    vd.EntityID,
				State:         hueState,
				VirtualDevice: vd,
			}
		}

		// Pre-sort devices by HueID (numeric)
		sorted := make([]*model.Device, 0, len(newDevices))
		for _, d := range newDevices {
			sorted = append(sorted, d)
		}
		sort.Slice(sorted, func(i, j int) bool {
			idI, _ := strconv.Atoi(sorted[i].ID)
			idJ, _ := strconv.Atoi(sorted[j].ID)
			return idI < idJ
		})

		s.devices = newDevices
		s.sortedDevices = sorted
		s.lastRefresh = time.Now()
		s.initialized = true
		s.logger.Info("Bridge: refreshed devices", "count", len(s.devices))
		return nil, nil
	})
	return err
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
	devices := make([]*model.Device, 0, len(s.sortedDevices))
	for _, d := range s.sortedDevices {
		devices = append(devices, s.copyDevice(d))
	}
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
	return s.translatorFactory.Metadata(deviceType)
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
			s.logger.Debug("Bridge: ON ignored, no-op configured", "device_id", id)
			s.mu.Unlock()
			return nil
		}
		if !tmpState.On && vd.ActionConfig.NoOpOff {
			s.logger.Debug("Bridge: OFF ignored, no-op configured", "device_id", id)
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
	cmd := s.translatorFactory.ToHA(device.Type, &tmpState, device.VirtualDevice)
	s.mu.Unlock()

	select {
	case s.workerSem <- struct{}{}:
		go func() {
			defer func() { <-s.workerSem }()
			err := s.haPort.SetState(context.Background(), deviceCopy, cmd)
			if err != nil {
				s.logger.Error("Error setting HA state", "error", err)
			}
		}()
	default:
		s.logger.Warn("Bridge: too many concurrent HA calls, request rejected", "device_id", id)
		return fmt.Errorf("too many concurrent requests, please try again later")
	}

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
	s.mu.Lock()
	s.lastRefresh = time.Now().Add(-5 * time.Second) // Ensure we can refresh
	s.mu.Unlock()

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
	filter := NewEntityFilter(ignored, s.logger)
	for _, st := range states {
		if !filter.Supports(st.EntityID) {
			continue
		}

		name := ""
		for _, attr := range nameAttributes {
			if n, ok := st.Attributes[attr].(string); ok {
				name = n
				break
			}
		}
		if name == "" {
			name = st.EntityID
		}

		entities = append(entities, ports.HomeAssistantEntity{
			EntityID:     st.EntityID,
			FriendlyName: name,
		})
	}
	return entities
}
