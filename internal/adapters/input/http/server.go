package http

import (
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
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
	mux.HandleFunc("/admin", s.handleAdmin)
	mux.HandleFunc("/admin/config", s.handleConfig)
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
<presentationURL>admin</presentationURL>
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

	subPath := parts[1:]
	if len(subPath) == 0 {
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

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Hue Bridge Emulator Admin</title>
    <style>
        body { font-family: sans-serif; max-width: 600px; margin: 40px auto; padding: 20px; line-height: 1.6; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input[type="text"], input[type="password"] { width: 100%; padding: 8px; margin-bottom: 20px; box-sizing: border-box; }
        button { padding: 10px 15px; background: #007bff; color: white; border: none; cursor: pointer; }
        button:hover { background: #0056b3; }
        #status { margin-top: 20px; padding: 10px; border-radius: 4px; display: none; }
        .success { background: #d4edda; color: #155724; }
        .error { background: #f8d7da; color: #721c24; }
    </style>
</head>
<body>
    <h1>Configuration</h1>
    <form id="configForm">
        <label for="hass_url">Home Assistant URL</label>
        <input type="text" id="hass_url" name="hass_url" placeholder="http://192.168.1.10:8123">

        <label for="hass_token">Long-Lived Access Token</label>
        <input type="password" id="hass_token" name="hass_token">

        <button type="submit">Save Configuration</button>
    </form>
    <div id="status"></div>

    <script>
        const form = document.getElementById('configForm');
        const status = document.getElementById('status');

        // Load current config
        fetch('/admin/config')
            .then(res => res.json())
            .then(data => {
                document.getElementById('hass_url').value = data.hass_url || '';
                document.getElementById('hass_token').value = data.hass_token || '';
            });

        form.onsubmit = async (e) => {
            e.preventDefault();
            const formData = new FormData(form);
            const data = Object.fromEntries(formData.entries());

            try {
                const res = await fetch('/admin/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
                if (res.ok) {
                    status.textContent = 'Configuration saved successfully!';
                    status.className = 'success';
                    status.style.display = 'block';
                } else {
                    throw new Error(await res.text());
                }
            } catch (err) {
                status.textContent = 'Error: ' + err.message;
                status.className = 'error';
                status.style.display = 'block';
            }
        };
    </script>
</body>
</html>
`)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cfg, err := s.bridge.GetConfig(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	} else if r.Method == "POST" {
		var cfg model.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err := s.bridge.UpdateConfig(r.Context(), &cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
