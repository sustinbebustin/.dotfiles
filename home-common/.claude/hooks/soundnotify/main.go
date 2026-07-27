// soundnotify plays Claude Code alert sounds on the machine you are sitting at
// rather than the machine Claude Code runs on.
//
// The workstation that owns the speakers runs `soundnotify serve`, which listens
// on a unix socket. Wherever Claude Code actually runs, hooks invoke
// `soundnotify arm|disarm|play`, which connect to that same socket path. When
// Claude Code runs over ssh, the two paths are stitched together by a remote
// forward in the client's ssh config:
//
//	RemoteForward /home/austin/.claude/run/sound.sock /Users/austin/.claude/run/sound.sock
//
// so neither side needs to know whether it is local or remote. With no forward
// and no server the client is a silent no-op, which is what makes it safe to
// wire into hooks unconditionally.
//
// `arm` schedules a sound instead of playing it immediately, and only fires if
// nothing disarms that session first. Claude Code's permission prompt arms the
// timer; the events that can only happen once you have answered it disarm the
// timer. An alert therefore reaches you only when a prompt has actually been
// left sitting unanswered.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"syscall"
	"time"
)

const (
	socketEnv    = "CLAUDE_SOUND_SOCKET"
	socketRel    = ".claude/run/sound.sock"
	audioRel     = ".claude/hooks/utils/audio"
	soundExt     = ".mp3"
	dialTimeout  = 250 * time.Millisecond
	replyTimeout = 500 * time.Millisecond
	readTimeout  = 2 * time.Second

	// Repeats of the same sound inside this window are dropped, so several
	// sessions alerting at once produce one alert instead of overlapping audio.
	coalesceWindow = 2 * time.Second

	maxArmDelay      = 10 * time.Minute
	maxRequestBytes  = 4 << 10
	maxHookInputSize = 1 << 20
)

// soundNamePattern keeps a sound name resolvable to exactly one file in the
// audio directory: no separators, no traversal, no extension games.
var soundNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type action string

const (
	actionArm    action = "arm"
	actionDisarm action = "disarm"
	actionPlay   action = "play"
)

// request is the wire format between client and server. Fields that do not
// apply to an action are absent, and validate rejects any other combination.
type request struct {
	Action  action `json:"action"`
	Session string `json:"session"`
	Sound   string `json:"sound,omitempty"`
	DelayMS int64  `json:"delay_ms,omitempty"`
}

func (r request) validate() error {
	if r.Session == "" {
		return errors.New("missing session")
	}
	switch r.Action {
	case actionArm:
		if r.Sound == "" {
			return errors.New("arm requires a sound")
		}
		if r.DelayMS < 0 || time.Duration(r.DelayMS)*time.Millisecond > maxArmDelay {
			return fmt.Errorf("arm delay must be between 0 and %s, got %dms", maxArmDelay, r.DelayMS)
		}
		return nil
	case actionPlay:
		if r.Sound == "" {
			return errors.New("play requires a sound")
		}
		if r.DelayMS != 0 {
			return errors.New("play does not take a delay; use arm")
		}
		return nil
	case actionDisarm:
		if r.Sound != "" || r.DelayMS != 0 {
			return errors.New("disarm takes no sound or delay")
		}
		return nil
	default:
		return fmt.Errorf("unknown action %q", r.Action)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if os.Args[1] == "serve" {
		if err := runServer(); err != nil {
			fmt.Fprintf(os.Stderr, "soundnotify: %v\n", err)
			os.Exit(1)
		}
		return
	}
	runClient(os.Args[1:])
}

const usage = `usage:
  soundnotify serve                 listen on the sound socket and play alerts
  soundnotify arm <sound> <delay>   play <sound> after <delay> unless disarmed
  soundnotify disarm                cancel this session's pending alert
  soundnotify play <sound>          play <sound> now

Client commands read the Claude Code hook payload on stdin to identify the
session, and are a silent no-op when no server is reachable.
`

// ---------------------------------------------------------------------------
// client
// ---------------------------------------------------------------------------

// runClient never exits non-zero. A hook that fails loudly would surface
// permission-denied errors or block a tool call over a missed sound effect, so
// every failure here is reported on stderr and otherwise swallowed on purpose.
func runClient(args []string) {
	req, err := parseClientArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "soundnotify: %v\n", err)
		return
	}
	if err := send(req); err != nil {
		fmt.Fprintf(os.Stderr, "soundnotify: %v\n", err)
	}
}

func parseClientArgs(args []string) (request, error) {
	req := request{Action: action(args[0]), Session: sessionID()}
	rest := args[1:]

	switch req.Action {
	case actionArm:
		if len(rest) != 2 {
			return request{}, fmt.Errorf("arm requires <sound> <delay>, got %d argument(s)", len(rest))
		}
		delay, err := time.ParseDuration(rest[1])
		if err != nil {
			return request{}, fmt.Errorf("invalid delay %q: %w", rest[1], err)
		}
		req.Sound, req.DelayMS = rest[0], delay.Milliseconds()
	case actionPlay:
		if len(rest) != 1 {
			return request{}, fmt.Errorf("play requires <sound>, got %d argument(s)", len(rest))
		}
		req.Sound = rest[0]
	case actionDisarm:
		if len(rest) != 0 {
			return request{}, fmt.Errorf("disarm takes no arguments, got %d", len(rest))
		}
	default:
		return request{}, fmt.Errorf("unknown command %q", req.Action)
	}

	if err := req.validate(); err != nil {
		return request{}, err
	}
	return req, nil
}

func send(req request) error {
	socket := socketPath()
	conn, err := net.DialTimeout("unix", socket, dialTimeout)
	if err != nil {
		// No forward and no server is the normal state on a host with no
		// speakers attached, so this is not worth a louder complaint.
		return fmt.Errorf("no sound server at %s (%w)", socket, err)
	}
	defer conn.Close()

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}
	if err := conn.SetDeadline(time.Now().Add(replyTimeout)); err != nil {
		return fmt.Errorf("setting deadline: %w", err)
	}
	if _, err := conn.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("sending %s: %w", req.Action, err)
	}

	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && reply == "" {
		return fmt.Errorf("no reply from sound server: %w", err)
	}
	if reply = trimNewline(reply); reply != "ok" {
		return fmt.Errorf("sound server refused %s: %s", req.Action, reply)
	}
	return nil
}

// sessionID reads the Claude Code hook payload on stdin so each session gets an
// independent timer. Falls back to a fixed key when invoked from a terminal.
func sessionID() string {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return "manual"
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, maxHookInputSize))
	if err != nil {
		return "manual"
	}
	var in struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(raw, &in); err != nil || in.SessionID == "" {
		return "manual"
	}
	return in.SessionID
}

// ---------------------------------------------------------------------------
// server
// ---------------------------------------------------------------------------

type server struct {
	audioDir string

	mu       sync.Mutex
	pending  map[string]*time.Timer
	lastPlay map[string]time.Time
}

func runServer() error {
	socket := socketPath()
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}
	if conn, err := net.DialTimeout("unix", socket, dialTimeout); err == nil {
		conn.Close()
		return fmt.Errorf("another soundnotify server is already listening on %s", socket)
	}
	// Nothing answered, so any socket still on disk is a leftover from an
	// unclean shutdown and would otherwise make Listen fail with EADDRINUSE.
	if err := os.Remove(socket); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing stale socket %s: %w", socket, err)
	}

	ln, err := net.Listen("unix", socket)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", socket, err)
	}
	defer ln.Close()
	if err := os.Chmod(socket, 0o600); err != nil {
		return fmt.Errorf("restricting socket permissions: %w", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		ln.Close()
	}()

	srv := &server{
		audioDir: filepath.Join(homeDir(), audioRel),
		pending:  make(map[string]*time.Timer),
		lastPlay: make(map[string]time.Time),
	}
	fmt.Fprintf(os.Stderr, "soundnotify: listening on %s, sounds from %s\n", socket, srv.audioDir)

	for {
		conn, err := ln.Accept()
		if err != nil {
			// Accept only fails permanently here because the signal handler
			// closed the listener, which is an orderly shutdown.
			return nil
		}
		go srv.handle(conn)
	}
}

func (s *server) handle(conn net.Conn) {
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(readTimeout)); err != nil {
		return
	}

	line, err := bufio.NewReader(io.LimitReader(conn, maxRequestBytes)).ReadString('\n')
	if err != nil && line == "" {
		return
	}

	if err := s.dispatch(line); err != nil {
		fmt.Fprintf(os.Stderr, "soundnotify: %v\n", err)
		fmt.Fprintf(conn, "error: %v\n", err)
		return
	}
	fmt.Fprintln(conn, "ok")
}

func (s *server) dispatch(line string) error {
	var req request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return fmt.Errorf("malformed request: %w", err)
	}
	if err := req.validate(); err != nil {
		return err
	}

	switch req.Action {
	case actionDisarm:
		s.disarm(req.Session)
		return nil
	case actionArm, actionPlay:
		// Resolve the file up front so a typo in a hook is reported to the
		// client immediately instead of silently failing when the timer fires.
		file, err := s.soundFile(req.Sound)
		if err != nil {
			return err
		}
		if req.Action == actionPlay {
			go s.play(req.Sound, file)
			return nil
		}
		s.arm(req.Session, req.Sound, file, time.Duration(req.DelayMS)*time.Millisecond)
		return nil
	default:
		return fmt.Errorf("unknown action %q", req.Action)
	}
}

func (s *server) arm(session, sound, file string, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.pending[session]; ok {
		existing.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(delay, func() {
		s.mu.Lock()
		// A disarm that lands while this timer is firing removes the entry, and
		// a re-arm replaces it. Either way this alert is stale, so drop it.
		if current, ok := s.pending[session]; !ok || current != timer {
			s.mu.Unlock()
			return
		}
		delete(s.pending, session)
		s.mu.Unlock()
		s.play(sound, file)
	})
	s.pending[session] = timer
}

func (s *server) disarm(session string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if timer, ok := s.pending[session]; ok {
		timer.Stop()
		delete(s.pending, session)
	}
}

func (s *server) play(sound, file string) {
	s.mu.Lock()
	if last, ok := s.lastPlay[sound]; ok && time.Since(last) < coalesceWindow {
		s.mu.Unlock()
		return
	}
	s.lastPlay[sound] = time.Now()
	s.mu.Unlock()

	cmd, err := playerCommand(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "soundnotify: %v\n", err)
		return
	}
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "soundnotify: playing %s failed: %v\n", file, err)
	}
}

func (s *server) soundFile(name string) (string, error) {
	if !soundNamePattern.MatchString(name) {
		return "", fmt.Errorf("invalid sound name %q: expected lowercase letters, digits and underscores", name)
	}
	file := filepath.Join(s.audioDir, name+soundExt)
	if _, err := os.Stat(file); err != nil {
		return "", fmt.Errorf("sound %q not found: no readable %s", name, file)
	}
	return file, nil
}

func playerCommand(file string) (*exec.Cmd, error) {
	if runtime.GOOS == "darwin" {
		return exec.Command("afplay", file), nil
	}
	players := []struct {
		bin  string
		args []string
	}{
		{"mpv", []string{"--no-video", "--really-quiet"}},
		{"ffplay", []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}},
		{"mpg123", []string{"-q"}},
	}
	for _, p := range players {
		bin, err := exec.LookPath(p.bin)
		if err != nil {
			continue
		}
		args := append(append([]string{}, p.args...), file)
		return exec.Command(bin, args...), nil
	}
	return nil, errors.New("no audio player found: install mpv, ffplay or mpg123")
}

// ---------------------------------------------------------------------------
// paths
// ---------------------------------------------------------------------------

func socketPath() string {
	if custom := os.Getenv(socketEnv); custom != "" {
		return custom
	}
	return filepath.Join(homeDir(), socketRel)
}

func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
