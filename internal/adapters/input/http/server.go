package http

import (
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/domain/translator"
	"hue-bridge-emulator/internal/ports"
	"net/http"
	"strings"
	"github.com/amimof/huego"
)

type Server struct {
	bridge            ports.BridgePort
	translatorFactory *translator.Factory
	ip                string
}

func NewServer(bridge ports.BridgePort, ip string) *Server {
	return &Server{
		bridge:            bridge,
		translatorFactory: translator.NewFactory(),
		ip:                ip,
	}
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/description.xml", s.handleDescription)
	mux.HandleFunc("/api", s.handleAPI)
	mux.HandleFunc("/api/", s.handleAPI)
	mux.HandleFunc("/admin", s.handleAdmin)
	mux.HandleFunc("/admin/config", s.handleConfig)
	mux.HandleFunc("/admin/ha-entities", s.handleHAEntities)
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
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `[{"success":{"username": "admin"}}]`)
}

func (s *Server) handleFullState(w http.ResponseWriter, r *http.Request) {
	devices, err := s.bridge.GetDevices(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lights := make(map[string]*huego.Light)
	for _, d := range devices {
		strategy := s.translatorFactory.GetTranslator(d.Type)
		meta := strategy.GetMetadata()
		lights[d.ID] = &huego.Light{
			Name:             d.Name,
			Type:             meta.Type,
			State:            d.State,
			ModelID:          meta.ModelID,
			UniqueID:         d.ID,
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fullState)
}

func (s *Server) handleGetLights(w http.ResponseWriter, r *http.Request) {
	devices, err := s.bridge.GetDevices(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lights := make(map[string]*huego.Light)
	for _, d := range devices {
		strategy := s.translatorFactory.GetTranslator(d.Type)
		meta := strategy.GetMetadata()
		lights[d.ID] = &huego.Light{
			Name:             d.Name,
			Type:             meta.Type,
			State:            d.State,
			ModelID:          meta.ModelID,
			UniqueID:         d.ID,
			ManufacturerName: meta.ManufacturerName,
		}
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

	strategy := s.translatorFactory.GetTranslator(device.Type)
	meta := strategy.GetMetadata()
	l := &huego.Light{
		Name:             device.Name,
		Type:             meta.Type,
		State:            device.State,
		ModelID:          meta.ModelID,
		UniqueID:         device.ID,
		ManufacturerName: meta.ManufacturerName,
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
        body { font-family: sans-serif; max-width: 1000px; margin: 40px auto; padding: 20px; line-height: 1.6; }
        .tabs { display: flex; border-bottom: 1px solid #ccc; margin-bottom: 20px; }
        .tab { padding: 10px 20px; cursor: pointer; border: 1px solid transparent; border-bottom: none; }
        .tab.active { border-color: #ccc; border-radius: 4px 4px 0 0; background: #f9f9f9; font-weight: bold; }
        .content { display: none; }
        .content.active { display: block; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input[type="text"], input[type="password"], select { width: 100%; padding: 8px; margin-bottom: 20px; box-sizing: border-box; }
        button { padding: 10px 15px; background: #007bff; color: white; border: none; cursor: pointer; }
        button:hover { background: #0056b3; }
        table { width: 100%; border-collapse: collapse; margin-bottom: 20px; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        #status { margin-top: 20px; padding: 10px; border-radius: 4px; display: none; }
        .success { background: #d4edda; color: #155724; }
        .error { background: #f8d7da; color: #721c24; }
    </style>
</head>
<body>
    <h1>Hue Bridge Emulator Admin</h1>
    <div class="tabs">
        <div class="tab active" onclick="showTab('general')">General Config</div>
        <div class="tab" onclick="showTab('ha-devices')">Home Assistant Devices</div>
    </div>

    <div id="general" class="content active">
        <form id="configForm">
            <label for="hass_url">Home Assistant URL</label>
            <input type="text" id="hass_url" name="hass_url" placeholder="http://192.168.1.10:8123">

            <label for="hass_token">Long-Lived Access Token</label>
            <input type="password" id="hass_token" name="hass_token">

            <button type="submit">Save General Config</button>
        </form>
    </div>

    <div id="ha-devices" class="content">
        <button onclick="loadEntities()">Refresh Entities</button>
        <p>Select entities to expose to Alexa and define their type.</p>
        <table id="entitiesTable">
            <thead>
                <tr>
                    <th>Expose</th>
                    <th>Entity ID</th>
                    <th>Name</th>
                    <th>Type</th>
                    <th>Custom Formulas (if Custom type)</th>
                </tr>
            </thead>
            <tbody></tbody>
        </table>
        <button onclick="saveMappings()">Save Device Mappings</button>
    </div>

    <div id="status"></div>

    <script>
        let currentConfig = { entity_mappings: {} };

        function showTab(id) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.content').forEach(c => c.classList.remove('active'));
            document.querySelector('.tab[onclick="showTab(\''+id+'\')"]').classList.add('active');
            document.getElementById(id).classList.add('active');
        }

        async function loadConfig() {
            const res = await fetch('/admin/config');
            currentConfig = await res.json();
            if (!currentConfig.entity_mappings) currentConfig.entity_mappings = {};
            document.getElementById('hass_url').value = currentConfig.hass_url || '';
            document.getElementById('hass_token').value = currentConfig.hass_token || '';
        }

        async function loadEntities() {
            const res = await fetch('/admin/ha-entities');
            const entities = await res.json();
            const tbody = document.querySelector('#entitiesTable tbody');
            tbody.innerHTML = '';

            entities.forEach(ent => {
                const mapping = currentConfig.entity_mappings[ent.entity_id] || ent;
                const tr = document.createElement('tr');
                const isCustom = mapping.type === 'custom';
                tr.innerHTML = '<td><input type="checkbox" class="exposed" data-id="' + ent.entity_id + '" ' + (mapping.exposed ? 'checked' : '') + '></td>' +
                    '<td>' + ent.entity_id + '</td>' +
                    '<td><input type="text" class="name" value="' + mapping.name + '"></td>' +
                    '<td>' +
                        '<select class="type">' +
                            '<option value="light" ' + (mapping.type === 'light' ? 'selected' : '') + '>Light</option>' +
                            '<option value="cover" ' + (mapping.type === 'cover' ? 'selected' : '') + '>Cover</option>' +
                            '<option value="climate" ' + (mapping.type === 'climate' ? 'selected' : '') + '>Climate</option>' +
                            '<option value="custom" ' + (mapping.type === 'custom' ? 'selected' : '') + '>Custom</option>' +
                        '</select>' +
                    '</td>' +
                    '<td>' +
                        '<div class="custom-fields" style="display: ' + (isCustom ? 'block' : 'none') + '">' +
                            'To Hue: <input type="text" class="to_hue" placeholder="x * 1" value="' + (mapping.custom_formula?.to_hue_formula || '') + '"><br>' +
                            'To HA: <input type="text" class="to_ha" placeholder="x / 1" value="' + (mapping.custom_formula?.to_ha_formula || '') + '"><br>' +
                            'On Service: <input type="text" class="on_service" placeholder="camera.enable_motion_detection" value="' + (mapping.custom_formula?.on_service || '') + '"><br>' +
                            'Off Service: <input type="text" class="off_service" placeholder="camera.disable_motion_detection" value="' + (mapping.custom_formula?.off_service || '') + '"><br>' +
                            'On Effect: <input type="text" class="on_effect" placeholder="domain.service" value="' + (mapping.custom_formula?.on_effect || '') + '"><br>' +
                            'Off Effect: <input type="text" class="off_effect" placeholder="domain.service" value="' + (mapping.custom_formula?.off_effect || '') + '">' +
                        '</div>' +
                    '</td>';

                tr.querySelector('.type').onchange = (e) => {
                    tr.querySelector('.custom-fields').style.display = e.target.value === 'custom' ? 'block' : 'none';
                };

                tbody.appendChild(tr);
            });
        }

        async function saveMappings() {
            const mappings = {};
            document.querySelectorAll('#entitiesTable tbody tr').forEach(tr => {
                const entity_id = tr.querySelector('.exposed').dataset.id;
                mappings[entity_id] = {
                    entity_id: entity_id,
                    hue_id: currentConfig.entity_mappings[entity_id]?.hue_id || '',
                    name: tr.querySelector('.name').value,
                    type: tr.querySelector('.type').value,
                    exposed: tr.querySelector('.exposed').checked,
                    custom_formula: {
                        to_hue_formula: tr.querySelector('.to_hue').value,
                        to_ha_formula: tr.querySelector('.to_ha').value,
                        on_service: tr.querySelector('.on_service').value,
                        off_service: tr.querySelector('.off_service').value,
                        on_effect: tr.querySelector('.on_effect').value,
                        off_effect: tr.querySelector('.off_effect').value
                    }
                };
            });
            currentConfig.entity_mappings = mappings;

            const res = await fetch('/admin/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(currentConfig)
            });
            showStatus(res.ok ? 'Mappings saved!' : 'Error saving mappings');
        }

        document.getElementById('configForm').onsubmit = async (e) => {
            e.preventDefault();
            currentConfig.hass_url = document.getElementById('hass_url').value;
            currentConfig.hass_token = document.getElementById('hass_token').value;
            const res = await fetch('/admin/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(currentConfig)
            });
            showStatus(res.ok ? 'General config saved!' : 'Error saving config');
        };

        function showStatus(msg) {
            const s = document.getElementById('status');
            s.textContent = msg;
            s.style.display = 'block';
            s.className = msg.includes('Error') ? 'error' : 'success';
        }

        loadConfig();
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
	}
}

func (s *Server) handleHAEntities(w http.ResponseWriter, r *http.Request) {
	entities, err := s.bridge.GetAllEntities(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entities)
}
