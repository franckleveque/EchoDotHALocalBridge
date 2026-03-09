package http

import (
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"net/http"
	"strings"

	"github.com/amimof/huego"
)

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if r.Method == "POST" && (path == "" || path == "/") {
		s.handleRegister(w, r)
		return
	}

	if len(parts) < 1 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	subPath := parts[1:]
	if len(subPath) == 0 {
		s.handleFullState(w, r)
		return
	}

	switch subPath[0] {
	case "lights":
		if len(subPath) == 1 {
			s.handleGetLights(w, r)
		} else if len(subPath) == 2 {
			s.handleGetLight(w, r, subPath[1])
		} else if len(subPath) == 3 && subPath[2] == "state" {
			s.handleSetLightState(w, r, subPath[1])
		}
	}
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, []map[string]interface{}{
		{
			"success": map[string]string{
				"username": "admin",
			},
		},
	})
}

func (s *Server) handleFullState(w http.ResponseWriter, r *http.Request) {
	devices, err := s.hue.GetDevices(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lights := make(map[string]*huego.Light)
	for _, d := range devices {
		meta := s.hue.GetDeviceMetadata(d.Type)
		lights[d.ID] = &huego.Light{
			Name:             d.Name,
			Type:             meta.Type,
			State:            s.toHueState(d.State),
			ModelID:          meta.ModelID,
			UniqueID:         s.formatUniqueID(d.ID),
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

	s.jsonResponse(w, fullState)
}

func (s *Server) handleGetLights(w http.ResponseWriter, r *http.Request) {
	devices, err := s.hue.GetDevices(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lights := make(map[string]*huego.Light)
	for _, d := range devices {
		meta := s.hue.GetDeviceMetadata(d.Type)
		lights[d.ID] = &huego.Light{
			Name:             d.Name,
			Type:             meta.Type,
			State:            s.toHueState(d.State),
			ModelID:          meta.ModelID,
			UniqueID:         s.formatUniqueID(d.ID),
			ManufacturerName: meta.ManufacturerName,
		}
	}

	s.jsonResponse(w, lights)
}

func (s *Server) handleGetLight(w http.ResponseWriter, r *http.Request, id string) {
	device, err := s.hue.GetDevice(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	meta := s.hue.GetDeviceMetadata(device.Type)
	l := &huego.Light{
		Name:             device.Name,
		Type:             meta.Type,
		State:            s.toHueState(device.State),
		ModelID:          meta.ModelID,
		UniqueID:         s.formatUniqueID(device.ID),
		ManufacturerName: meta.ManufacturerName,
	}

	s.jsonResponse(w, l)
}

func (s *Server) handleSetLightState(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "PUT" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rawState map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawState); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stateUpdate := &model.DeviceState{}
	if on, ok := rawState["on"].(bool); ok {
		stateUpdate.On = on
	}
	if bri, ok := rawState["bri"].(float64); ok {
		stateUpdate.Bri = uint8(bri)
		stateUpdate.UpdatedByBri = true
	}
	// TODO: handle other fields if needed, but for now these are the main ones

	err := s.hue.UpdateDeviceState(r.Context(), id, stateUpdate)
	if err != nil {
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

	s.jsonResponse(w, resp)
}
