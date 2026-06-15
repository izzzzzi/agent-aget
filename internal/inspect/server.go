// Package inspect provides a lightweight HTTP+SSE server for monitoring aget
// sessions in real time. It reads session records and snapshots from disk and
// streams updates to connected clients. Zero dependencies beyond the stdlib.
package inspect

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/snapshot"
	"github.com/izzzzzi/agent-aget/internal/state"
)

// Server is the inspect HTTP server.
type Server struct {
	port      int
	sessions  *session.Registry
	snapshots *snapshot.Store
	html      string // embedded dashboard HTML
}

// New creates an inspect server that reads from the aget state directories.
func New(port int) *Server {
	return &Server{
		port:      port,
		sessions:  session.NewRegistry(state.SessionsDir()),
		snapshots: snapshot.NewStore(state.SnapshotsDir()),
		html:      dashboardHTML,
	}
}

// ListenAndServe starts the HTTP server on the configured port.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/snapshot/", s.handleSnapshot)
	mux.HandleFunc("/api/stream", s.handleStream)
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("inspect server listening on http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(s.html))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	records, err := s.sessions.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, records)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Path[len("/api/snapshot/"):]
	if sid == "" {
		http.Error(w, "sid required", http.StatusBadRequest)
		return
	}
	rec, err := s.snapshots.Load(sid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, rec)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Track last known state to only send deltas.
	var prevSessionIDs []string
	var mu sync.Mutex

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			mu.Lock()
			records, err := s.sessions.List()
			if err != nil {
				mu.Unlock()
				continue
			}
			ids := make([]string, 0, len(records))
			changed := false
			for _, rec := range records {
				ids = append(ids, rec.SID)
				if rec.UpdatedAt.After(time.Now().Add(-3 * time.Second)) {
					changed = true
				}
			}
			if !changed && len(ids) == len(prevSessionIDs) {
				mu.Unlock()
				continue
			}
			prevSessionIDs = ids

			data, _ := json.Marshal(map[string]any{
				"sessions": records,
				"time":     time.Now().UTC(),
			})
			mu.Unlock()
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// dashboardHTML is the embedded inspect dashboard.
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>aget inspect</title>
<style>
body{font-family:system-ui,sans-serif;margin:2rem;background:#1a1a2e;color:#eee;}
h1{color:#4fc3f7}
.sessions{display:grid;gap:1rem;grid-template-columns:repeat(auto-fill,minmax(320px,1fr))}
.card{background:#16213e;border-radius:8px;padding:1rem;border:1px solid #0f3460}
.card h3{margin:0 0 .5rem}
.card .url{color:#90caf9;font-size:.85rem;word-break:break-all}
.card .meta{color:#999;font-size:.8rem;margin-top:.5rem}
.card .actions{display:flex;gap:.5rem;margin-top:.75rem}
.card .actions a{padding:.3rem .6rem;background:#0f3460;color:#4fc3f7;border-radius:4px;text-decoration:none;font-size:.8rem}
.card .actions a:hover{background:#1a5276}
.snap{background:#0d2137;border-radius:6px;padding:.75rem;margin-top:.75rem}
.snap pre{font-size:.75rem;max-height:200px;overflow:auto;color:#bbb}
#log{font-family:monospace;font-size:.75rem;background:#0d1117;padding:.5rem;border-radius:4px;max-height:120px;overflow:auto;margin-top:1rem}
#log div{margin:.1rem 0}
</style></head>
<body>
<h1>aget inspect</h1>
<p>Streaming session state from <code>AGET_STATE_DIR</code></p>
<div id="sessions" class="sessions"></div>
<div id="log"></div>
<script>
const es = new EventSource('/api/stream');
const sessionsDiv = document.getElementById('sessions');
const logDiv = document.getElementById('log');
function log(msg){const d=document.createElement('div');d.textContent=new Date().toLocaleTimeString()+' '+msg;logDiv.prepend(d);if(logDiv.children.length>50)logDiv.lastChild.remove()}
es.onmessage = (e) => {
  const data = JSON.parse(e.data);
  sessionsDiv.innerHTML = '';
  for (const s of data.sessions) {
    const card = document.createElement('div'); card.className = 'card';
    const url = s.url || 'no url';
    card.innerHTML = '<h3>' + (s.name || s.sid) + '</h3><div class="url">' + url + '</div><div class="meta">SID: ' + s.sid + '<br>Created: ' + new Date(s.created_at).toLocaleString() + '</div><div class="actions"><a href="/api/snapshot/' + s.sid + '">Snapshot JSON</a><a href="javascript:void(0)" onclick="loadSnapshot(\''+s.sid+'\',this)">View Snapshot</a></div><div id="snap-'+s.sid+'" class="snap" style="display:none"><pre>loading...</pre></div>';
    sessionsDiv.appendChild(card);
  }
  log('refreshed ' + data.sessions.length + ' sessions');
};
async function loadSnapshot(sid, el) {
  const snap = document.getElementById('snap-'+sid);
  snap.style.display = 'block';
  snap.querySelector('pre').textContent = 'loading...';
  try {
    const r = await fetch('/api/snapshot/'+sid);
    const j = await r.json();
    snap.querySelector('pre').textContent = JSON.stringify(j, null, 2);
  } catch(e) { snap.querySelector('pre').textContent = 'error: '+e; }
}
</script>
</body></html>`
