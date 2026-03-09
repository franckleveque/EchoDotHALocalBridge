package http

import (
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleAdminSetup(w http.ResponseWriter, r *http.Request) {
	if s.authService.Exists() {
		// Cleanup the limiter map once setup is done to free memory
		s.limiterMu.Lock()
		if len(s.setupLimiter) > 0 {
			s.setupLimiter = make(map[string]time.Time)
		}
		s.limiterMu.Unlock()

		http.Error(w, "Forbidden - Setup already completed", http.StatusForbidden)
		return
	}

	// Rate limiting for setup
	ip := s.getClientIP(r)
	s.limiterMu.Lock()
	if last, ok := s.setupLimiter[ip]; ok && time.Since(last) < 2*time.Second {
		s.limiterMu.Unlock()
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}

	// Basic cleanup: if map gets too large, clear it
	if len(s.setupLimiter) > 1000 {
		for k, v := range s.setupLimiter {
			if time.Since(v) > 10*time.Minute {
				delete(s.setupLimiter, k)
			}
		}
	}

	s.setupLimiter[ip] = time.Now()
	s.limiterMu.Unlock()

	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, adminSetupHTML)
	} else if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		if len(username) < 3 || len(password) < 8 {
			http.Error(w, "Invalid username or password length", http.StatusBadRequest)
			return
		}

		if err := s.authService.CreateCredentials(r.Context(), username, password); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, adminHTML)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cfg, err := s.admin.GetConfig(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Mask token for frontend
		displayCfg := struct {
			HassURL             string                 `json:"hass_url"`
			HassToken           string                 `json:"hass_token"`
			HassTokenConfigured bool                   `json:"hass_token_configured"`
			VirtualDevices      []*model.VirtualDevice `json:"virtual_devices"`
		}{
			HassURL:             cfg.HassURL,
			HassToken:           "",
			HassTokenConfigured: cfg.HassToken != "",
			VirtualDevices:      cfg.VirtualDevices,
		}

		s.jsonResponse(w, displayCfg)
	} else if r.Method == "POST" {
		var newCfg model.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// If token is empty, keep the existing one
		if newCfg.HassToken == "" {
			currentCfg, err := s.admin.GetConfig(r.Context())
			if err == nil {
				newCfg.HassToken = currentCfg.HassToken
			}
		}

		err := s.admin.UpdateConfig(r.Context(), &newCfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) handleHAEntities(w http.ResponseWriter, r *http.Request) {
	entities, err := s.admin.GetAllEntities(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, entities)
}

func (s *Server) getClientIP(r *http.Request) string {
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func (s *Server) handleAdminTestAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VirtualDevice *model.VirtualDevice `json:"virtual_device"`
		StateUpdate   *model.DeviceState   `json:"state_update"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := s.admin.TestDeviceAction(r.Context(), req.VirtualDevice, req.StateUpdate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

const adminSetupHTML = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Hue Bridge Emulator - Setup</title>
    <style>
        body { font-family: sans-serif; max-width: 500px; margin: 100px auto; padding: 20px; line-height: 1.6; background-color: #f4f4f9; }
        .card { background: white; padding: 30px; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
        h1 { margin-top: 0; color: #333; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input[type="text"], input[type="password"] { width: 100%; padding: 10px; margin-bottom: 20px; box-sizing: border-box; border: 1px solid #ccc; border-radius: 4px; }
        button { width: 100%; padding: 12px; background: #007bff; color: white; border: none; cursor: pointer; border-radius: 4px; font-size: 16px; }
        button:hover { background: #0056b3; }
        .error { color: #dc3545; margin-bottom: 15px; }
    </style>
</head>
<body>
    <div class="card">
        <h1>Initial Setup</h1>
        <p>Define your administrative credentials to secure the bridge.</p>
        <form method="POST">
            <label for="username">Username</label>
            <input type="text" id="username" name="username" required minlength="3">

            <label for="password">Password</label>
            <input type="password" id="password" name="password" required minlength="8">

            <button type="submit">Create Account</button>
        </form>
    </div>
</body>
</html>
`

const adminHTML = `
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
                    <th>Test Actions</th>
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
            <select id="dev_type" onchange="toggleAdvanced()">
                <option value="light">Light</option>
                <option value="cover">Cover</option>
                <option value="climate">Climate</option>
                <option value="custom">Custom</option>
            </select>

            <div id="modal_test_actions" style="margin-bottom: 20px; padding: 10px; border: 1px dashed #007bff; border-radius: 4px;">
                <label>Test Current Device (Real-time)</label>
                <div style="display: flex; gap: 10px;">
                    <button type="button" onclick="testCurrentForm({on: true})">On</button>
                    <button type="button" onclick="testCurrentForm({on: false})">Off</button>
                    <button type="button" onclick="testCurrentForm({bri: 127})">Dim 50%</button>
                </div>
            </div>

            <fieldset id="advanced_config">
                <legend>Custom Actions Configuration</legend>
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
            document.getElementById('hass_token').value = '';
            const tokenInput = document.getElementById('hass_token');
            if (config.hass_token_configured) {
                tokenInput.placeholder = 'Token already set (leave empty to keep current)';
            } else {
                tokenInput.placeholder = 'Enter Long-Lived Access Token';
            }

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

            // Show all entities without filtering
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
                const hueId = vd.hue_id || '';
                const testButtons = hueId ?
                    '<button onclick="testAction(\''+hueId+'\', {on: true})">On</button> ' +
                    '<button onclick="testAction(\''+hueId+'\', {on: false})">Off</button> ' +
                    '<button onclick="testAction(\''+hueId+'\', {bri: 127})">Dim 50%</button>' :
                    '<span style="color: #666; font-style: italic;">Save config first</span>';

                tr.innerHTML =
                    '<td>' + (hueId || 'new') + '</td>' +
                    '<td>' + vd.name + '</td>' +
                    '<td>' + vd.entity_id + '</td>' +
                    '<td>' + vd.type + '</td>' +
                    '<td>' + testButtons + '</td>' +
                    '<td>' +
                        '<button onclick="openDeviceModal(' + index + ')">Edit</button> ' +
                        '<button class="delete" onclick="deleteDevice(' + index + ')">Delete</button>' +
                    '</td>';
                tbody.appendChild(tr);
            });
        }

        async function testAction(hueId, state) {
            try {
                const res = await fetch('/api/admin/lights/' + hueId + '/state', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(state)
                });
                if (res.ok) {
                    showStatus('Action sent successfully');
                } else {
                    showStatus('Error sending action');
                }
            } catch (e) {
                showStatus('Error: ' + e.message);
            }
        }

        async function testCurrentForm(stateUpdate) {
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

            const vd = {
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

            try {
                const res = await fetch('/admin/test-action', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        virtual_device: vd,
                        state_update: stateUpdate
                    })
                });
                if (res.ok) {
                    showStatus('Test action sent successfully');
                } else {
                    showStatus('Error sending test action');
                }
            } catch (e) {
                showStatus('Error: ' + e.message);
            }
        }

        function toggleAdvanced() {
            const type = document.getElementById('dev_type').value;
            const advContainer = document.getElementById('advanced_config');
            advContainer.style.display = (type === 'custom') ? 'block' : 'none';
            renderEntitySelect(document.getElementById('dev_entity').value);
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
            toggleAdvanced();
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
            // Note: If hass_token is empty, the backend will preserve the current one
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
`
