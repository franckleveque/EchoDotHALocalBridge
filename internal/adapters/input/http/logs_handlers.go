package http

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleLogsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(adminLogsHTML))
}

func (s *Server) handleLogsData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lines := []string{}
	if s.logs != nil {
		if l := s.logs.Lines(); l != nil {
			lines = l
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(lines)
}

// Log values come from untrusted sources (for example HA error bodies), so the
// page only ever assigns them through textContent.
const adminLogsHTML = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Hue Bridge Emulator - Logs</title>
    <style>
        body { font-family: sans-serif; max-width: 1100px; margin: 20px auto; padding: 0 20px; background-color: #f4f4f9; }
        .toolbar { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; margin-bottom: 10px; }
        input[type="text"] { padding: 8px; border: 1px solid #ccc; border-radius: 4px; min-width: 240px; }
        button { padding: 8px 16px; background: #007bff; color: white; border: none; cursor: pointer; border-radius: 4px; }
        button:hover { background: #0056b3; }
        pre { background: #1e1e1e; color: #ddd; padding: 12px; border-radius: 4px; height: 70vh; overflow: auto; white-space: pre-wrap; word-break: break-all; }
    </style>
</head>
<body>
    <h1>Logs</h1>
    <p><a href="/admin">Back to admin</a></p>
    <div class="toolbar">
        <input type="text" id="filter" placeholder="Filter lines" aria-label="Filter lines">
        <button type="button" id="refresh">Refresh</button>
        <label><input type="checkbox" id="auto"> Auto-refresh (5 s)</label>
        <span id="info"></span>
    </div>
    <pre id="logs"></pre>
    <script>
        var lines = [];
        var timer = null;
        var pre = document.getElementById('logs');
        var filterEl = document.getElementById('filter');
        var info = document.getElementById('info');

        function render() {
            var q = filterEl.value.toLowerCase();
            var shown = lines.filter(function (l) { return q === '' || l.toLowerCase().indexOf(q) !== -1; });
            var atBottom = pre.scrollTop + pre.clientHeight >= pre.scrollHeight - 20;
            pre.textContent = shown.join('\n');
            info.textContent = shown.length + ' / ' + lines.length + ' lines';
            if (atBottom) { pre.scrollTop = pre.scrollHeight; }
        }

        function load() {
            fetch('/admin/logs/data', { credentials: 'same-origin', cache: 'no-store' })
                .then(function (r) {
                    if (!r.ok) { throw new Error('HTTP ' + r.status); }
                    return r.json();
                })
                .then(function (data) { lines = data; render(); })
                .catch(function (e) { info.textContent = 'Error: ' + e.message; });
        }

        document.getElementById('refresh').addEventListener('click', load);
        filterEl.addEventListener('input', render);
        document.getElementById('auto').addEventListener('change', function (e) {
            if (timer) { clearInterval(timer); timer = null; }
            if (e.target.checked) { load(); timer = setInterval(load, 5000); }
        });
        load();
    </script>
</body>
</html>
`
