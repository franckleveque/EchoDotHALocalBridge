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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Hue Bridge Emulator Admin</title>
    <style>
        body { font-family: sans-serif; max-width: 1200px; margin: 40px auto; padding: 20px; line-height: 1.6; background-color: #f4f4f9; }
        .tabs { display: flex; border-bottom: 2px solid #007bff; margin-bottom: 20px; }
        .tab { padding: 10px 20px; cursor: pointer; border: 1px solid transparent; border-bottom: none; }
        .tab.active { border-color: #007bff; border-radius: 4px 4px 0 0; background: white; font-weight: bold; color: #007bff; }
        .content { display: none; padding: 20px; background: white; border: 1px solid #ccc; border-radius: 0 0 4px 4px; }
        .content.active { display: block; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input[type="text"], input[type="password"], select, textarea { width: 100%; padding: 8px; margin-bottom: 10px; box-sizing: border-box; border: 1px solid #ccc; border-radius: 4px; }
        button { padding: 10px 15px; background: #007bff; color: white; border: none; cursor: pointer; border-radius: 4px; }
        button:hover { background: #0056b3; }
        button.delete { background: #dc3545; }
        button.delete:hover { background: #c82333; }
        table { width: 100%; border-collapse: collapse; margin-bottom: 20px; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background-color: #f8f9fa; }
        .action-config { font-size: 0.9em; background: #fdfdfe; padding: 10px; border: 1px dashed #ccc; margin-top: 5px; }
        #status { margin-top: 20px; padding: 10px; border-radius: 4px; display: none; position: fixed; bottom: 20px; right: 20px; z-index: 1000; }
        .success { background: #d4edda; color: #155724; border: 1px solid #c3e6cb; }
        .error { background: #f8d7da; color: #721c24; border: 1px solid #f5c6cb; }
        .modal { display: none; position: fixed; z-index: 1001; left: 0; top: 0; width: 100%; height: 100%; background-color: rgba(0,0,0,0.5); overflow-y: auto; }
        .modal-content { background-color: white; margin: 2vh auto; padding: 20px; border: 1px solid #888; width: 90%; max-width: 600px; border-radius: 8px; max-height: 90vh; overflow-y: auto; }
    </style>
</head>
<body>
    <h1>Hue Bridge Emulator Admin</h1>
    <div class="tabs">
        <div class="tab active" onclick="showTab('general')">General Config</div>
        <div class="tab" onclick="showTab('virtual-devices')">Virtual Devices</div>
    </div>

    <div id="general" class="content active">
        <h2>Connection</h2>
        <form id="configForm">
            <label for="hass_url">Home Assistant URL</label>
            <input type="text" id="hass_url" name="hass_url" placeholder="http://192.168.1.10:8123">

            <label for="hass_token">Long-Lived Access Token</label>
            <input type="password" id="hass_token" name="hass_token">

            <button type="submit">Save General Config</button>
        </form>
    </div>

    <div id="virtual-devices" class="content">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
            <h2>Virtual Devices Mapping</h2>
            <button onclick="openDeviceModal()">+ Add Virtual Device</button>
        </div>
        <table id="devicesTable">
            <thead>
                <tr>
                    <th>HueID</th>
                    <th>Alexa Name</th>
                    <th>HA Entity ID</th>
                    <th>Type</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody></tbody>
        </table>
        <button onclick="saveAll()">Save Configuration</button>
    </div>

    <div id="deviceModal" class="modal">
        <div class="modal-content">
            <h2 id="modalTitle">Device Configuration</h2>
            <input type="hidden" id="edit_index">
            <label>Alexa Name</label>
            <input type="text" id="dev_name" placeholder="e.g. Salon Chauffage">
            <label>HA Entity ID</label>
            <div style="display: flex; gap: 5px;">
                <select id="dev_entity" style="flex-grow: 1;">
                    <option value="">-- Select an Entity --</option>
                </select>
                <button type="button" onclick="loadEntities()" title="Refresh entities" style="padding: 5px 10px; margin-bottom: 10px;">Refresh</button>
            </div>
            <label>Type</label>
            <select id="dev_type">
                <option value="light">Light</option>
                <option value="cover">Cover</option>
                <option value="climate">Climate</option>
                <option value="custom">Custom</option>
            </select>

            <fieldset>
                <legend>Actions Configuration</legend>
                <label>ON Service</label>
                <input type="text" id="on_service" placeholder="homeassistant.turn_on">
                <label>ON Payload (JSON)</label>
                <textarea id="on_payload" placeholder='{"brightness": 255}'></textarea>
                <label><input type="checkbox" id="no_op_on"> No-Op for ON</label>

                <hr>
                <label>OFF Service</label>
                <input type="text" id="off_service" placeholder="homeassistant.turn_off">
                <label>OFF Payload (JSON)</label>
                <textarea id="off_payload" placeholder='{}'></textarea>
                <label><input type="checkbox" id="no_op_off"> No-Op for OFF</label>

                <hr>
                <label>DIM: To Hue formula (variable: x)</label>
                <input type="text" id="to_hue" placeholder="x * 2.54">
                <label>DIM: To HA formula (variable: x)</label>
                <input type="text" id="to_ha" placeholder="x / 2.54">

                <hr>
                <label><input type="checkbox" id="omit_eid"> Omit entity_id in calls</label>
            </fieldset>

            <div style="margin-top: 20px; text-align: right;">
                <button onclick="closeDeviceModal()">Cancel</button>
                <button onclick="applyDeviceChanges()">Apply</button>
            </div>
        </div>
    </div>

    <div id="status"></div>

    <script>
        let config = { virtual_devices: [] };
        let allEntities = [];

        function showTab(id) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.content').forEach(c => c.classList.remove('active'));
            const tab = document.querySelector('.tab[onclick="showTab(\''+id+'\')"]');
            if (tab) tab.classList.add('active');
            document.getElementById(id).classList.add('active');
        }

        async function loadData() {
            const res = await fetch('/admin/config');
            config = await res.json();
            if (!config.virtual_devices) config.virtual_devices = [];

            document.getElementById('hass_url').value = config.hass_url || '';
            document.getElementById('hass_token').value = config.hass_token || '';

            renderDevices();
            loadEntities();
        }

        async function loadEntities() {
            try {
                const res = await fetch('/admin/ha-entities');
                if (!res.ok) throw new Error('Failed to fetch entities');
                allEntities = await res.json();
                allEntities.sort((a, b) => (a.friendly_name || '').localeCompare(b.friendly_name || ''));
                renderEntitySelect();
            } catch (e) {
                console.error('Error loading entities:', e);
            }
        }

        function renderEntitySelect(selectedValue = '') {
            const sel = document.getElementById('dev_entity');

            sel.innerHTML = '<option value="">-- Select an Entity --</option>';

            // Add existing entities from discovery
            allEntities.forEach(e => {
                const opt = document.createElement('option');
                opt.value = e.entity_id;
                opt.textContent = e.friendly_name + ' (' + e.entity_id + ')';
                sel.appendChild(opt);
            });

            // If selectedValue is not in discovered entities, add it as a placeholder
            if (selectedValue && !allEntities.find(e => e.entity_id === selectedValue)) {
                const opt = document.createElement('option');
                opt.value = selectedValue;
                opt.textContent = '⚠️ ' + selectedValue + ' (unreachable)';
                sel.appendChild(opt);
            }

            if (selectedValue) sel.value = selectedValue;
        }

        function renderDevices() {
            const tbody = document.querySelector('#devicesTable tbody');
            tbody.innerHTML = '';
            config.virtual_devices.forEach((vd, index) => {
                const tr = document.createElement('tr');
                tr.innerHTML =
                    '<td>' + (vd.hue_id || 'new') + '</td>' +
                    '<td>' + vd.name + '</td>' +
                    '<td>' + vd.entity_id + '</td>' +
                    '<td>' + vd.type + '</td>' +
                    '<td>' +
                        '<button onclick="openDeviceModal(' + index + ')">Edit</button> ' +
                        '<button class="delete" onclick="deleteDevice(' + index + ')">Delete</button>' +
                    '</td>';
                tbody.appendChild(tr);
            });
        }

        function openDeviceModal(index = -1) {
            document.getElementById('edit_index').value = index;
            if (index >= 0) {
                const d = config.virtual_devices[index];
                document.getElementById('dev_name').value = d.name;
                renderEntitySelect(d.entity_id);
                document.getElementById('dev_type').value = d.type;
                const ac = d.action_config || {};
                document.getElementById('on_service').value = ac.on_service || '';
                document.getElementById('on_payload').value = JSON.stringify(ac.on_payload || {}, null, 2);
                document.getElementById('no_op_on').checked = ac.no_op_on || false;
                document.getElementById('off_service').value = ac.off_service || '';
                document.getElementById('off_payload').value = JSON.stringify(ac.off_payload || {}, null, 2);
                document.getElementById('no_op_off').checked = ac.no_op_off || false;
                document.getElementById('to_hue').value = ac.to_hue_formula || '';
                document.getElementById('to_ha').value = ac.to_ha_formula || '';
                document.getElementById('omit_eid').checked = ac.omit_entity_id || false;
                document.getElementById('modalTitle').textContent = 'Edit Virtual Device';
            } else {
                document.getElementById('dev_name').value = '';
                renderEntitySelect('');
                document.getElementById('dev_type').value = 'light';
                document.getElementById('on_service').value = '';
                document.getElementById('on_payload').value = '{}';
                document.getElementById('no_op_on').checked = false;
                document.getElementById('off_service').value = '';
                document.getElementById('off_payload').value = '{}';
                document.getElementById('no_op_off').checked = false;
                document.getElementById('to_hue').value = '';
                document.getElementById('to_ha').value = '';
                document.getElementById('omit_eid').checked = false;
                document.getElementById('modalTitle').textContent = 'Add Virtual Device';
            }
            document.getElementById('deviceModal').style.display = 'block';
        }

        function closeDeviceModal() {
            document.getElementById('deviceModal').style.display = 'none';
        }

        function applyDeviceChanges() {
            const index = parseInt(document.getElementById('edit_index').value);
            let on_payload, off_payload;
            try {
                on_payload = JSON.parse(document.getElementById('on_payload').value || '{}');
            } catch (e) {
                alert('Invalid ON Payload JSON: ' + e.message);
                return;
            }
            try {
                off_payload = JSON.parse(document.getElementById('off_payload').value || '{}');
            } catch (e) {
                alert('Invalid OFF Payload JSON: ' + e.message);
                return;
            }

            const d = {
                name: document.getElementById('dev_name').value,
                entity_id: document.getElementById('dev_entity').value,
                type: document.getElementById('dev_type').value,
                action_config: {
                    on_service: document.getElementById('on_service').value,
                    on_payload: on_payload,
                    no_op_on: document.getElementById('no_op_on').checked,
                    off_service: document.getElementById('off_service').value,
                    off_payload: off_payload,
                    no_op_off: document.getElementById('no_op_off').checked,
                    to_hue_formula: document.getElementById('to_hue').value,
                    to_ha_formula: document.getElementById('to_ha').value,
                    omit_entity_id: document.getElementById('omit_eid').checked
                }
            };
            if (index >= 0) {
                d.hue_id = config.virtual_devices[index].hue_id;
                config.virtual_devices[index] = d;
            } else {
                config.virtual_devices.push(d);
            }
            renderDevices();
            closeDeviceModal();
        }

        function deleteDevice(index) {
            if (confirm('Delete this virtual device?')) {
                config.virtual_devices.splice(index, 1);
                renderDevices();
            }
        }

        async function saveAll() {
            config.hass_url = document.getElementById('hass_url').value;
            config.hass_token = document.getElementById('hass_token').value;
            const res = await fetch('/admin/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            });
            if (res.ok) {
                showStatus('Configuration saved and applied!');
                await loadEntities();
            } else {
                showStatus('Error saving config');
            }
        }

        document.getElementById('configForm').onsubmit = async (e) => {
            e.preventDefault();
            await saveAll();
        };

        function showStatus(msg) {
            const s = document.getElementById('status');
            s.textContent = msg;
            s.style.display = 'block';
            s.className = msg.includes('Error') ? 'error' : 'success';
            setTimeout(() => { s.style.display = 'none'; }, 3000);
        }

        loadData();
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
