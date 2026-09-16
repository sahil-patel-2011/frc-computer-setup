package wizard

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/engine"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/host"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

type Server struct {
	Catalog *manifest.Catalog
	Runner  *engine.Runner
	Host    host.Host
	Demo    bool

	mu      sync.Mutex
	running bool
	acks    map[string]chan string
	subs    []chan engine.Event
}

func New(catalog *manifest.Catalog, runner *engine.Runner, h host.Host, demo bool) *Server {
	if runner.Emit == nil {
		runner.Emit = func(engine.Event) {}
	}
	s := &Server{Catalog: catalog, Runner: runner, Host: h, Demo: demo, acks: map[string]chan string{}}
	prev := runner.Emit
	runner.Emit = func(e engine.Event) {
		prev(e)
		s.broadcast(e)
	}
	return s
}

func (s *Server) broadcast(e engine.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(web)))
	mux.HandleFunc("/api/catalog", s.catalog)
	mux.HandleFunc("/api/run", s.run)
	mux.HandleFunc("/api/ack", s.ack)
	mux.HandleFunc("/api/events", s.events)
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	return mux
}

func (s *Server) ListenAndServe() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	url := "http://" + ln.Addr().String() + "/"
	go func() {
		srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("wizard server: %v", err)
		}
	}()
	return url, nil
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	goos, goarch := "linux", "amd64"
	if s.Host != nil {
		goos, goarch = s.Host.GOOS(), s.Host.GOARCH()
	}
	type tool struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Summary     string `json:"summary"`
		Why         string `json:"why"`
		Group       string `json:"group"`
		Kind        string `json:"kind"`
		Selected    bool   `json:"selected"`
		Available   bool   `json:"available"`
		WindowsOnly bool   `json:"windowsOnly,omitempty"`
		VendorURL   string `json:"vendorUrl,omitempty"`
		Version     string `json:"version,omitempty"`
		Size        int64  `json:"size,omitempty"`
		Reason      string `json:"reason,omitempty"`
		Installed   bool   `json:"installed,omitempty"`
		HaveVersion string `json:"haveVersion,omitempty"`
		Current     bool   `json:"current,omitempty"`
		Prompt      string `json:"prompt,omitempty"`
	}
	out := struct {
		Season int    `json:"season"`
		Demo   bool   `json:"demo"`
		OS     string `json:"os"`
		Arch   string `json:"arch"`
		Tools  []tool `json:"tools"`
	}{Season: s.Catalog.Season, Demo: s.Demo, OS: goos, Arch: goarch}
	for _, t := range s.Catalog.Tools {
		resolved := t.Resolve(goos, goarch)
		kind := t.EffectiveKind(goos, goarch)
		available := kind != "windows_only" && kind != "unavailable"
		selected := available && t.SelectedByDefault()
		if t.ID == "vscode" {
			selected = false
		}
		item := tool{
			ID: t.ID, Name: t.Name, Summary: t.Summary, Why: t.Why,
			Group: t.Group, Kind: kind, Selected: selected,
			Available: available, WindowsOnly: t.WindowsOnly, VendorURL: resolved.VendorURL,
		}
		det := engine.DetectTool(s.Host, resolved)
		item.Installed = det.Installed
		item.HaveVersion = det.Version
		item.Current = det.Current
		if t.ID == "vscode" {
			item.Prompt = "Install VS Code?"
			if engine.WPILibVSCodeInstalled(s.Host, s.Catalog.Season) {
				item.Reason = "WPILib already ships VS Code. Leave this off unless you want a separate Microsoft VS Code."
			} else {
				item.Reason = "Install VS Code? Default is no. WPILib’s VS Code is enough for robot code."
			}
		}
		if det.Installed && item.Reason == "" {
			if det.Current {
				item.Reason = "Already current on this computer. Uncheck to skip; leave checked only if you want the walkthrough to offer Update."
			} else {
				item.Reason = "Already found. The walkthrough will ask Update or skip — it will not reinstall unless you say Update."
			}
		}
		if !available && t.WindowsOnly {
			item.Reason = "Windows only — FRC Driver Station does not run here."
			if t.ID != "ni-game-tools" {
				item.Reason = "Windows only on this computer."
			}
		}
		if kind == "unavailable" {
			item.Reason = "No official installer for this OS/arch. Not inventing a URL."
		}
		if resolved.Pinned != nil {
			item.Version = resolved.Pinned.Version
			item.Size = resolved.Pinned.Size
		}
		out.Tools = append(out.Tools, item)
	}
	writeJSON(w, out)
}

func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ToolIDs []string `json:"toolIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		http.Error(w, "already running", http.StatusConflict)
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()
		for _, id := range body.ToolIDs {
			tool, ok := s.Catalog.Tool(id)
			if !ok {
				s.broadcast(engine.Event{ToolID: id, Phase: engine.PhaseError, Message: "Unknown tool", Err: "unknown"})
				continue
			}
			s.mu.Lock()
			ch := make(chan string, 1)
			s.acks[id] = ch
			s.mu.Unlock()
			s.Runner.RunTool(tool, ch)
		}
		s.broadcast(engine.Event{ToolID: "", Name: "", Phase: engine.PhaseDone, Message: "Setup walkthrough finished."})
	}()
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) ack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ToolID string `json:"toolId"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	ch := s.acks[body.ToolID]
	s.mu.Unlock()
	if ch != nil {
		select {
		case ch <- body.Action:
		default:
		}
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no sse", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan engine.Event, 32)
	s.mu.Lock()
	s.subs = append(s.subs, ch)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		out := s.subs[:0]
		for _, existing := range s.subs {
			if existing != ch {
				out = append(out, existing)
			}
		}
		s.subs = out
		s.mu.Unlock()
	}()
	fmt.Fprintf(w, "event: hello\ndata: {\"ok\":true}\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func LaunchInAppWindow(h host.Host, url string) error {
	return h.OpenURL(url)
}
