package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/luischavesdev/society/internal/agent"
	"github.com/luischavesdev/society/internal/models"
)

// DaemonState holds runtime info about the daemon process, persisted to disk.
type DaemonState struct {
	PID       int       `json:"pid"`
	Agents    []string  `json:"agents"`
	Ports     []int     `json:"ports"`
	AgentsDir string    `json:"agents_dir"`
	StartedAt time.Time `json:"started_at"`
}

// DaemonStart launches the daemon as a background process by re-execing
// the binary with the "run" subcommand and SOCIETY_DAEMON_CHILD=1.
func DaemonStart(agentsDir string, names []string, out io.Writer) error {
	if state, err := readDaemonState(); err == nil && isProcessAlive(state.PID) {
		return fmt.Errorf("daemon already running (PID %d)", state.PID)
	}

	// Validate configs before backgrounding so the user gets fast feedback.
	configs, err := discoverConfigs(agentsDir)
	if err != nil {
		return err
	}
	if len(names) > 0 {
		configs, err = filterConfigs(configs, names)
		if err != nil {
			return err
		}
	}
	if err := checkPortConflicts(configs); err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}

	args := []string{exe, "daemon", "run", "--agents", agentsDir}
	args = append(args, names...)

	dir, err := societyDir()
	if err != nil {
		return err
	}
	logPath := filepath.Join(dir, "daemon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer logFile.Close()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return fmt.Errorf("opening /dev/null: %w", err)
	}
	defer devNull.Close()

	attr := &os.ProcAttr{
		Env:   append(os.Environ(), "SOCIETY_DAEMON_CHILD=1"),
		Files: []*os.File{devNull, logFile, logFile},
	}

	proc, err := os.StartProcess(exe, args, attr)
	if err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}
	proc.Release()

	// Brief wait for the child to write its PID file.
	time.Sleep(500 * time.Millisecond)

	state, err := readDaemonState()
	if err != nil {
		return fmt.Errorf("daemon may have failed to start, check %s", logPath)
	}

	fmt.Fprintf(out, "Daemon started (PID %d)\n", state.PID)
	for i, name := range state.Agents {
		fmt.Fprintf(out, "  %s on :%d\n", name, state.Ports[i])
	}
	return nil
}

// DaemonRun starts agents in the foreground. It blocks until interrupted.
func DaemonRun(agentsDir string, names []string, out io.Writer) error {
	configs, err := discoverConfigs(agentsDir)
	if err != nil {
		return err
	}
	if len(names) > 0 {
		configs, err = filterConfigs(configs, names)
		if err != nil {
			return err
		}
	}
	if len(configs) == 0 {
		return fmt.Errorf("no agent configs found in %s", agentsDir)
	}
	if err := checkPortConflicts(configs); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return runAgents(ctx, configs, agentsDir, out)
}

// runAgents is the shared core that both DaemonRun uses directly.
func runAgents(ctx context.Context, configs []*models.AgentConfig, agentsDir string, out io.Writer) error {
	state := &DaemonState{
		PID:       os.Getpid(),
		AgentsDir: agentsDir,
		StartedAt: time.Now(),
	}
	for _, cfg := range configs {
		state.Agents = append(state.Agents, cfg.Name)
		state.Ports = append(state.Ports, cfg.Port)
	}

	if err := writeDaemonState(state); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}
	defer removePIDFile()

	// Print status banner.
	fmt.Fprintf(out, "Daemon running (PID %d)\n", state.PID)
	for i, cfg := range configs {
		fmt.Fprintf(out, "  %s on :%d\n", cfg.Name, state.Ports[i])
	}
	fmt.Fprintf(out, "%d agents starting\n", len(configs))

	var wg sync.WaitGroup
	errCh := make(chan error, len(configs))

	for _, cfg := range configs {
		h, err := agent.NewHandler(cfg)
		if err != nil {
			return fmt.Errorf("creating handler for %s: %w", cfg.Name, err)
		}

		card := models.AgentCard{
			Name:        cfg.Name,
			Description: cfg.Description,
			URL:         fmt.Sprintf("http://localhost:%d", cfg.Port),
			Skills:      cfg.Skills,
		}

		srv := agent.NewServer(card, h)
		addr := fmt.Sprintf(":%d", cfg.Port)

		wg.Add(1)
		go func(name, addr string) {
			defer wg.Done()
			if err := srv.Start(ctx, addr); err != nil {
				errCh <- fmt.Errorf("agent %s: %w", name, err)
			}
		}(cfg.Name, addr)
	}

	wg.Wait()
	close(errCh)

	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// DaemonStop sends SIGTERM to a running daemon and waits for it to exit.
func DaemonStop(out io.Writer) error {
	state, err := readDaemonState()
	if err != nil {
		return fmt.Errorf("daemon not running (no PID file)")
	}

	if !isProcessAlive(state.PID) {
		removePIDFile()
		return fmt.Errorf("daemon not running (stale PID file, cleaned up)")
	}

	proc, err := os.FindProcess(state.PID)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", state.PID, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("sending SIGTERM to %d: %w", state.PID, err)
	}

	fmt.Fprintf(out, "Sent SIGTERM to daemon (PID %d)\n", state.PID)

	// Wait up to 5 seconds for the process to exit.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !isProcessAlive(state.PID) {
			fmt.Fprintln(out, "Daemon stopped")
			removePIDFile()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Fprintln(out, "Daemon still running after 5s; it may stop shortly")
	return nil
}

// DaemonStatus prints information about the running daemon.
func DaemonStatus(out io.Writer) error {
	state, err := readDaemonState()
	if err != nil {
		fmt.Fprintln(out, "Daemon: not running")
		return nil
	}

	if !isProcessAlive(state.PID) {
		removePIDFile()
		fmt.Fprintln(out, "Daemon: not running (stale PID file)")
		return nil
	}

	uptime := time.Since(state.StartedAt).Truncate(time.Second)
	fmt.Fprintf(out, "Daemon: running (uptime: %s) [PID %d]\n", formatDuration(uptime), state.PID)
	for i, name := range state.Agents {
		fmt.Fprintf(out, "  %s on :%d\n", name, state.Ports[i])
	}
	fmt.Fprintf(out, "%d agents active\n", len(state.Agents))
	return nil
}

// --- helpers ---

func societyDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	dir := filepath.Join(home, ".society")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating society directory: %w", err)
	}
	return dir, nil
}

func pidFilePath() (string, error) {
	dir, err := societyDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.json"), nil
}

func readDaemonState() (*DaemonState, error) {
	path, err := pidFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing PID file: %w", err)
	}
	return &state, nil
}

func writeDaemonState(state *DaemonState) error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func removePIDFile() error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func discoverConfigs(dir string) ([]*models.AgentConfig, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("globbing configs: %w", err)
	}

	// Also match .yml files.
	ymlMatches, err := filepath.Glob(filepath.Join(dir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("globbing configs: %w", err)
	}
	matches = append(matches, ymlMatches...)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no agent configs found in %s", dir)
	}

	var configs []*models.AgentConfig
	for _, path := range matches {
		cfg, err := agent.LoadConfig(path)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", filepath.Base(path), err)
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}

func filterConfigs(configs []*models.AgentConfig, names []string) ([]*models.AgentConfig, error) {
	byName := make(map[string]*models.AgentConfig, len(configs))
	for _, cfg := range configs {
		byName[cfg.Name] = cfg
	}

	var filtered []*models.AgentConfig
	for _, name := range names {
		cfg, ok := byName[name]
		if !ok {
			available := make([]string, 0, len(byName))
			for k := range byName {
				available = append(available, k)
			}
			return nil, fmt.Errorf("unknown agent %q (available: %s)", name, strings.Join(available, ", "))
		}
		filtered = append(filtered, cfg)
	}
	return filtered, nil
}

func checkPortConflicts(configs []*models.AgentConfig) error {
	seen := make(map[int]string, len(configs))
	for _, cfg := range configs {
		if prev, ok := seen[cfg.Port]; ok {
			return fmt.Errorf("port %d conflict: %s and %s", cfg.Port, prev, cfg.Name)
		}
		seen[cfg.Port] = cfg.Name
	}
	return nil
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, int(d.Seconds())%60)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
