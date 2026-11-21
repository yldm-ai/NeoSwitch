package daemon

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	attachPath = "/neoswitch/attach"
	detachPath = "/neoswitch/detach"
	statusPath = "/neoswitch/status"
)

// Options tune the lifetime rules; zero values select the production defaults.
type Options struct {
	// StartupGrace is how long the daemon waits for its first session before giving up.
	StartupGrace time.Duration
	// PollInterval is how often attached Claude Code processes are checked.
	PollInterval time.Duration
	// Alive reports whether a process exists; tests replace it.
	Alive func(pid int) bool
	// Backend describes the upstream this daemon serves, reported by Status.
	Backend string
	// Version is the NeoSwitch version of the daemon binary, reported by Status so a session started after an update can tell it is served by the earlier version.
	Version string
	// Release is called with a project once no attached session uses it any more, so the next session there starts on Claude. It runs while attaches wait, so a session turning the project on again lands after it. A session attached without a project releases none.
	Release func(project string)
}

// Info is what a running daemon reports about itself. Version is empty for daemons from versions that did not report it.
type Info struct {
	Sessions int    `json:"sessions"`
	Backend  string `json:"backend"`
	Version  string `json:"version,omitempty"`
}

// Run serves proxy on s.Listen until every attached Claude Code process has exited, or until StartupGrace passes without any attach. Lifetime is tied to the sessions rather than an idle timer, so a session that sits idle for hours keeps working and nothing outlives the last session.
func Run(ctx context.Context, s State, proxy http.Handler, opts Options) error {
	if opts.StartupGrace == 0 {
		opts.StartupGrace = time.Minute
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = 10 * time.Second
	}
	if opts.Alive == nil {
		opts.Alive = processAlive
	}
	listener, err := net.Listen("tcp", s.Listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.Listen, err)
	}
	if opts.Release == nil {
		opts.Release = func(string) {}
	}
	sessions := &sessionSet{pids: map[int]map[string]bool{}, release: opts.Release}
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+attachPath, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, s.Token) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body struct {
			PID     int    `json:"pid"`
			Project string `json:"project"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil || body.PID <= 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// A daemon that decided to stop refuses, so Attach starts a new one instead of attaching to one about to exit.
		if !sessions.add(body.PID, body.Project) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Daemons that predate project tracking answer 204, which tells a session its project stays routed.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(attachReply{Tracked: body.Project != ""})
	})
	mux.HandleFunc("POST "+detachPath, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, s.Token) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body struct {
			PID int `json:"pid"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || body.PID <= 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sessions.remove(body.PID)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET "+statusPath, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, s.Token) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Info{Sessions: sessions.len(), Backend: opts.Backend, Version: opts.Version})
	})
	mux.Handle("/", proxy)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()
	deadline := time.Now().Add(opts.StartupGrace)
	for {
		select {
		case err := <-served:
			return err
		case <-ctx.Done():
			// Stopped from outside, as at logout or shutdown: the sessions go too, so their projects return to Claude.
			sessions.stop()
			return shutdown(server)
		case <-ticker.C:
			if sessions.prune(opts.Alive, time.Now().After(deadline)) {
				return shutdown(server)
			}
		}
	}
}

func shutdown(server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

type attachReply struct {
	// Tracked: the daemon turns the session's project off once no attached session uses it.
	Tracked bool `json:"tracked"`
}

// sessionSet holds the attached Claude Code processes and the projects each one turned on.
type sessionSet struct {
	mu       sync.Mutex
	pids     map[int]map[string]bool
	attached bool
	// closing: the daemon decided to stop and takes no more sessions.
	closing bool
	release func(project string)
}

// add reports false when the daemon is closing.
func (s *sessionSet) add(pid int, project string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	if s.pids[pid] == nil {
		s.pids[pid] = map[string]bool{}
	}
	if project != "" {
		s.pids[pid][project] = true
	}
	s.attached = true
	return true
}

func (s *sessionSet) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pids)
}

// remove forgets a session that ended, releasing the projects no other session uses.
func (s *sessionSet) remove(pid int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drop(pid)
}

// prune drops exited processes, releasing their projects, and reports whether the daemon should stop: every attached session has exited, or none attached before graceOver. Deciding to stop also refuses later attaches.
func (s *sessionSet) prune(alive func(int) bool, graceOver bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid := range s.pids {
		if !alive(pid) {
			s.drop(pid)
		}
	}
	if (!s.attached && graceOver) || (s.attached && len(s.pids) == 0) {
		s.closing = true
	}
	return s.closing
}

// stop releases every project and refuses later attaches.
func (s *sessionSet) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closing = true
	for pid := range s.pids {
		s.drop(pid)
	}
}

// drop removes pid and releases each of its projects that no remaining session uses. The caller holds mu.
func (s *sessionSet) drop(pid int) {
	projects := s.pids[pid]
	delete(s.pids, pid)
	for project := range projects {
		if !s.inUse(project) {
			s.release(project)
		}
	}
}

func (s *sessionSet) inUse(project string) bool {
	for _, projects := range s.pids {
		if projects[project] {
			return true
		}
	}
	return false
}

func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	// EPERM means the process exists but belongs to someone else, which still counts as alive.
	return err == nil || errors.Is(err, syscall.EPERM)
}

func authorized(r *http.Request, token string) bool {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	value, ok := strings.CutPrefix(auth, "Bearer ")
	return ok && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(value)), []byte(token)) == 1
}

// Status asks a running daemon how many sessions it serves and which backend it uses; ok is false when no NeoSwitch daemon answers on s.Listen with s.Token.
func Status(s State) (info Info, ok bool) {
	req, _ := http.NewRequest(http.MethodGet, s.BaseURL()+statusPath, nil)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := (&http.Client{Timeout: time.Second}).Do(req)
	if err != nil {
		return Info{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Info{}, false
	}
	if json.NewDecoder(resp.Body).Decode(&info) != nil {
		return Info{}, false
	}
	return info, true
}

// PortInUseError means another program holds the proxy's saved port, so no NeoSwitch daemon can listen there.
type PortInUseError struct {
	Listen string
}

func (e *PortInUseError) Error() string { return "another program is using " + e.Listen }

// portBusyWait is how long a held port may stay silent before it is blamed on another program. A NeoSwitch daemon that is starting or shutting down holds the port briefly without answering the status probe.
var portBusyWait = 2 * time.Second

// Attach registers a Claude Code process with the daemon, starting the daemon through spawn first when none is running, and reports whether the daemon tracks project: it then turns the project off once no attached session uses it. project is "" for a session that turns no project on. It tries three times because a daemon that is shutting down (its last session just exited) can still answer the status probe and then refuse the attach. A port held by another program fails at once with *PortInUseError instead of waiting for a daemon that can never listen.
func Attach(s State, pid int, project string, spawn func() error) (tracked bool, err error) {
	for attempt := 0; attempt < 3; attempt++ {
		if tracked, err = attachOnce(s, pid, project, spawn); err == nil {
			return tracked, nil
		}
		var busy *PortInUseError
		if errors.As(err, &busy) {
			return false, err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false, err
}

// Detach tells the daemon a session ended, so the projects only it used return to Claude at once rather than at the daemon's next check, which a session reopened right away would beat. A daemon that does not answer, or predates detaching, is left to that check.
func Detach(s State, pid int) {
	body, _ := json.Marshal(map[string]int{"pid": pid})
	req, _ := http.NewRequest(http.MethodPost, s.BaseURL()+detachPath, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

// portFree reports whether nothing listens on addr.
func portFree(addr string) bool {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

// answersWithin reports whether a NeoSwitch daemon answers on s within d.
func answersWithin(s State, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if _, ok := Status(s); ok {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func attachOnce(s State, pid int, project string, spawn func() error) (bool, error) {
	if _, ok := Status(s); !ok && !portFree(s.Listen) {
		if !answersWithin(s, portBusyWait) {
			return false, &PortInUseError{Listen: s.Listen}
		}
	} else if !ok {
		if err := spawn(); err != nil {
			return false, fmt.Errorf("start NeoSwitch daemon: %w", err)
		}
		deadline := time.Now().Add(15 * time.Second)
		for {
			if _, ok := Status(s); ok {
				break
			}
			if time.Now().After(deadline) {
				return false, fmt.Errorf("NeoSwitch daemon did not come up on %s", s.Listen)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	body, _ := json.Marshal(map[string]any{"pid": pid, "project": project})
	req, _ := http.NewRequest(http.MethodPost, s.BaseURL()+attachPath, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return false, fmt.Errorf("attach session: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNoContent:
		return false, nil
	case http.StatusOK:
		var reply attachReply
		if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
			return false, fmt.Errorf("attach session: %w", err)
		}
		return reply.Tracked, nil
	}
	return false, fmt.Errorf("attach session: daemon answered HTTP %d", resp.StatusCode)
}
