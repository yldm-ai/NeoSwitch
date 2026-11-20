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
	sessions := &sessionSet{pids: map[int]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+attachPath, func(w http.ResponseWriter, r *http.Request) {
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
		sessions.add(body.PID)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET "+statusPath, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, s.Token) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"sessions": sessions.len()})
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
			return shutdown(server)
		case <-ticker.C:
			attached, alive := sessions.prune(opts.Alive)
			if (!attached && time.Now().After(deadline)) || (attached && alive == 0) {
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

type sessionSet struct {
	mu       sync.Mutex
	pids     map[int]bool
	attached bool
}

func (s *sessionSet) add(pid int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pids[pid] = true
	s.attached = true
}

func (s *sessionSet) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pids)
}

// prune drops exited processes and reports whether any session ever attached and how many are still running.
func (s *sessionSet) prune(alive func(int) bool) (bool, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid := range s.pids {
		if !alive(pid) {
			delete(s.pids, pid)
		}
	}
	return s.attached, len(s.pids)
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

// Status asks a running daemon how many sessions it serves; ok is false when no NeoSwitch daemon answers on s.Listen with s.Token.
func Status(s State) (sessions int, ok bool) {
	req, _ := http.NewRequest(http.MethodGet, s.BaseURL()+statusPath, nil)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := (&http.Client{Timeout: time.Second}).Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, false
	}
	var body struct {
		Sessions int `json:"sessions"`
	}
	if json.NewDecoder(resp.Body).Decode(&body) != nil {
		return 0, false
	}
	return body.Sessions, true
}

// Attach registers a Claude Code process with the daemon, starting the daemon through spawn first when none is running. It tries twice because a daemon that is shutting down (its last session just exited) can still answer the status probe and then refuse the attach.
func Attach(s State, pid int, spawn func() error) error {
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if err = attachOnce(s, pid, spawn); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

func attachOnce(s State, pid int, spawn func() error) error {
	if _, ok := Status(s); !ok {
		if err := spawn(); err != nil {
			return fmt.Errorf("start NeoSwitch daemon: %w", err)
		}
		deadline := time.Now().Add(15 * time.Second)
		for {
			if _, ok := Status(s); ok {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("NeoSwitch daemon did not come up on %s", s.Listen)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	body, _ := json.Marshal(map[string]int{"pid": pid})
	req, _ := http.NewRequest(http.MethodPost, s.BaseURL()+attachPath, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("attach session: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("attach session: daemon answered HTTP %d", resp.StatusCode)
	}
	return nil
}
