package http

import (
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/ports"
	"net/http"
	"strings"
	"github.com/amimof/huego"
)

type Server struct {
	bridge ports.BridgePort
	ip     string
}

func NewServer(bridge ports.BridgePort, ip string) *Server {
	return &Server{
		bridge: bridge,
		ip:     ip,
	}
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/description.xml", s.handleDescription)
	mux.HandleFunc("/api", s.handleAPI)
	mux.HandleFunc("/api/", s.handleAPI)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleDescription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8" ?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
<specVersion>
<major>1</major>
<minor>0</minor>
</specVersion>
<URLBase>http://%s:80/</URLBase>
<device>
<deviceType>urn:schemas-upnp-org:device:Basic:1</deviceType>
<friendlyName>Philips hue (%s)</friendlyName>
<manufacturer>Royal Philips Electronics</manufacturer>
<manufacturerURL>http://www.philips.com</manufacturerURL>
<modelDescription>Philips hue Personal Wireless Lighting</modelDescription>
<modelName>Philips hue bridge 2012</modelName>
<modelNumber>929000226503</modelNumber>
<modelURL>http://www.meethue.com</modelURL>
<serialNumber>001788102201</serialNumber>
<UDN>uuid:2f402f80-da50-11e1-9b23-001788102201</UDN>
<presentationURL>index.html</presentationURL>
</device>
</root>`, s.ip, s.ip)
}

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

	// For Hue V1 emulation, we often ignore the username part
	subPath := parts[1:]
	if len(subPath) == 0 {
		// Return full state? Alexa usually asks for /lights
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
	// Alexa expects a username in return
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `[{"success":{"username": "admin"}}]`)
}

func (s *Server) handleGetLights(w http.ResponseWriter, r *http.Request) {
	devices, err := s.bridge.GetDevices(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lights := make(map[string]*huego.Light)
	for _, d := range devices {
		l := &huego.Light{
			ID:    0, // Will be set by ID in map
			Name:  d.Name,
			Type:  "Extended color light",
			State: d.State,
			ModelID: "LCT001",
			UniqueID: d.ID,
			ManufacturerName: "Philips",
		}
		lights[d.ID] = l
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lights)
}

func (s *Server) handleGetLight(w http.ResponseWriter, r *http.Request, id string) {
	device, err := s.bridge.GetDevice(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	l := &huego.Light{
		Name:  device.Name,
		Type:  "Extended color light",
		State: device.State,
		ModelID: "LCT001",
		UniqueID: device.ID,
		ManufacturerName: "Philips",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(l)
}

func (s *Server) handleSetLightState(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "PUT" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var stateUpdate map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&stateUpdate); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := s.bridge.UpdateDeviceState(r.Context(), id, stateUpdate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Hue API expects success response
	resp := []map[string]interface{}{}
	for k, v := range stateUpdate {
		resp = append(resp, map[string]interface{}{
			"success": map[string]interface{}{
				fmt.Sprintf("/lights/%s/state/%s", id, k): v,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
