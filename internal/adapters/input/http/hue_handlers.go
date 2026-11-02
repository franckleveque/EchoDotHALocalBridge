package http

import (
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/amimof/huego"
)

// handleAPIUnknown answers paths under /api that no route claims. Hue clients
// probe several of them, so the historical empty 200 answer is kept.
// HueHandlers serves the Hue bridge API emulated for Alexa.
type HueHandlers struct {
	hue    ports.HueEmulationPort
	logger *slog.Logger
}

func NewHueHandlers(hue ports.HueEmulationPort, logger *slog.Logger) *HueHandlers {
	return &HueHandlers{hue: hue, logger: logger}
}

func (h *HueHandlers) formatUniqueID(id string) string {
	numericID, _ := strconv.Atoi(id)
	return fmt.Sprintf("00:17:88:01:00:%02x:%02x:%02x-0b", (numericID>>16)&0xFF, (numericID>>8)&0xFF, numericID&0xFF)
}

func (h *HueHandlers) toHueState(ds *model.DeviceState) *huego.State {
	if ds == nil {
		return nil
	}
	return &huego.State{
		On:        ds.On,
		Bri:       ds.Bri,
		Hue:       ds.Hue,
		Sat:       ds.Sat,
		Xy:        ds.Xy,
		Ct:        ds.Ct,
		Reachable: ds.Reachable,
	}
}

func (h *HueHandlers) handleAPIUnknown(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *HueHandlers) handleRegister(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, []map[string]interface{}{
		{
			"success": map[string]string{
				"username": "admin",
			},
		},
	})
}

func (h *HueHandlers) handleFullState(w http.ResponseWriter, r *http.Request) {
	devices, err := h.hue.GetDevices(r.Context())
	if err != nil {
		h.logger.Error("HTTP: Hue request failed", "path", r.URL.Path, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lights := make(map[string]*huego.Light)
	for _, d := range devices {
		meta := h.hue.GetDeviceMetadata(d.Type)
		lights[d.ID] = &huego.Light{
			Name:             d.Name,
			Type:             meta.Type,
			State:            h.toHueState(d.State),
			ModelID:          meta.ModelID,
			UniqueID:         h.formatUniqueID(d.ID),
			ManufacturerName: meta.ManufacturerName,
		}
	}

	fullState := map[string]interface{}{
		"lights": lights,
		"groups": make(map[string]interface{}),
		"config": map[string]interface{}{
			"name":       "Philips hue",
			"swversion":  "01003542",
			"apiversion": "1.11.0",
			"mac":        "00:17:88:10:22:01",
			"bridgeid":   "001788FFFE102201",
			"modelid":    "BSB001",
		},
	}

	writeJSON(w, fullState)
}

func (h *HueHandlers) handleGetLights(w http.ResponseWriter, r *http.Request) {
	devices, err := h.hue.GetDevices(r.Context())
	if err != nil {
		h.logger.Error("HTTP: Hue request failed", "path", r.URL.Path, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lights := make(map[string]*huego.Light)
	for _, d := range devices {
		meta := h.hue.GetDeviceMetadata(d.Type)
		lights[d.ID] = &huego.Light{
			Name:             d.Name,
			Type:             meta.Type,
			State:            h.toHueState(d.State),
			ModelID:          meta.ModelID,
			UniqueID:         h.formatUniqueID(d.ID),
			ManufacturerName: meta.ManufacturerName,
		}
	}

	writeJSON(w, lights)
}

func (h *HueHandlers) handleGetLight(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	device, err := h.hue.GetDevice(r.Context(), id)
	if err != nil {
		h.logger.Debug("Hue: light not found", "id", id, "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	meta := h.hue.GetDeviceMetadata(device.Type)
	l := &huego.Light{
		Name:             device.Name,
		Type:             meta.Type,
		State:            h.toHueState(device.State),
		ModelID:          meta.ModelID,
		UniqueID:         h.formatUniqueID(device.ID),
		ManufacturerName: meta.ManufacturerName,
	}

	writeJSON(w, l)
}

func (h *HueHandlers) handleSetLightState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if r.Method != "PUT" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rawState map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawState); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stateUpdate := parseStateUpdate(rawState)
	// TODO: handle other fields if needed, but for now these are the main ones

	err := h.hue.UpdateDeviceState(r.Context(), id, stateUpdate)
	if err != nil {
		h.logger.Error("HTTP: Hue request failed", "path", r.URL.Path, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := []map[string]interface{}{}
	for k, v := range rawState {
		resp = append(resp, map[string]interface{}{
			"success": map[string]interface{}{
				fmt.Sprintf("/lights/%s/state/%s", id, k): v,
			},
		})
	}

	writeJSON(w, resp)
}
