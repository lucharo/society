package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/luischavesdev/society/internal/cli"
	"github.com/luischavesdev/society/internal/mcp"
)

// Set via ldflags: -ldflags "-X main.version=1.2.3"
var version = "dev"

//go:embed skills
var skillsFS embed.FS

func main() {
	mcp.Version = version

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	registryPath := os.Getenv("SOCIETY_REGISTRY")
	if registryPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			registryPath = filepath.Join(home, ".society", "registry.json")
		} else {
			registryPath = "registry.json"
		}
	}

	// Check for global --registry flag before subcommand
	for i, arg := range os.Args[1:] {
		if arg == "--registry" && i+2 < len(os.Args) {
			registryPath = os.Args[i+2]
			// Remove the flag from args
			os.Args = append(os.Args[:i+1], os.Args[i+3:]...)
			break
		}
		if strings.HasPrefix(arg, "--registry=") {
			registryPath = strings.TrimPrefix(arg, "--registry=")
			os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
			break
		}
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "onboard":
		onboardFlags := flag.NewFlagSet("onboard", flag.ExitOnError)
		manualFlag := onboardFlags.Bool("manual", false, "Interactive manual setup")
		deepFlag := onboardFlags.Bool("deep", false, "Probe SSH/Docker hosts for live A2A agents")
		onboardFlags.Bool("auto", false, "No-op: auto-detect is now the default")
		onboardFlags.Parse(os.Args[2:])

		if *manualFlag {
			err = cli.Onboard(registryPath, os.Stdin, os.Stdout)
		} else {
			err = cli.OnboardAuto(registryPath, cli.ScanOptions{Deep: *deepFlag}, os.Stdin, os.Stdout)
		}

	case "list":
		err = cli.List(registryPath, os.Stdout)

	case "remove":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: society remove <name>")
			os.Exit(1)
		}
		err = cli.Remove(registryPath, os.Args[2], os.Stdin, os.Stdout)

	case "ping":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: society ping <name>")
			os.Exit(1)
		}
		err = cli.Ping(registryPath, os.Args[2], os.Stdout)

	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		configPath := fs.String("config", "", "Agent config file path")
		stdio := fs.Bool("stdio", false, "Run in STDIO mode")
		fs.Parse(os.Args[2:])

		if *configPath == "" {
			fmt.Fprintln(os.Stderr, "usage: society run --config <path> [--stdio]")
			os.Exit(1)
		}
		err = cli.Run(*configPath, *stdio, os.Stdout)

	case "send":
		parsed := parseSendArgs(os.Args[2:])
		if len(parsed.positional) < 2 {
			fmt.Fprintln(os.Stderr, "usage: society send <name> <message> [--thread <id>] [--trace]")
			os.Exit(1)
		}
		message := strings.Join(parsed.positional[1:], " ")
		err = cli.Send(registryPath, parsed.positional[0], message, os.Stdout, parsed.trace, parsed.thread)

	case "export":
		fs := flag.NewFlagSet("export", flag.ExitOnError)
		outputPath := fs.String("output", "", "Output file path (default: stdout)")
		fs.Parse(os.Args[2:])
		err = cli.Export(registryPath, *outputPath, os.Stdout)

	case "import":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: society import <path-or-url>")
			os.Exit(1)
		}
		err = cli.Import(registryPath, os.Args[2], os.Stdin, os.Stdout)

	case "discover":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: society discover <url>")
			os.Exit(1)
		}
		err = cli.Discover(registryPath, os.Args[2], os.Stdin, os.Stdout)

	case "mcp":
		err = cli.MCP(registryPath, os.Stdin, os.Stdout)

	case "daemon":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: society daemon <start|stop|status|run> [agents...] [--agents <dir>]")
			os.Exit(1)
		}

		subcmd := os.Args[2]
		fs := flag.NewFlagSet("daemon", flag.ExitOnError)
		agentsDir := fs.String("agents", "agents", "Directory containing agent configs")
		fs.Parse(os.Args[3:])
		names := fs.Args()

		switch subcmd {
		case "start":
			err = cli.DaemonStart(*agentsDir, names, os.Stdout)
		case "stop":
			err = cli.DaemonStop(os.Stdout)
		case "status":
			err = cli.DaemonStatus(os.Stdout)
		case "run":
			err = cli.DaemonRun(*agentsDir, names, os.Stdout)
		default:
			fmt.Fprintf(os.Stderr, "unknown daemon subcommand: %s\n", subcmd)
			os.Exit(1)
		}

	case "version":
		fmt.Fprintf(os.Stdout, "society v%s\n", version)

	case "update":
		err = cli.Update(version, os.Stdout)

	case "skill":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: society skill <install|update>")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "install", "update":
			sub, _ := fs.Sub(skillsFS, "skills")
			err = cli.SkillInstall(sub, os.Stdout)
		default:
			fmt.Fprintf(os.Stderr, "unknown skill subcommand: %s\n", os.Args[2])
			os.Exit(1)
		}

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// sendParsed holds the result of parsing "society send" arguments.
type sendParsed struct {
	positional []string
	trace      bool
	thread     string
}

// parseSendArgs extracts flags from any position in the arg list.
// Go's flag package stops at the first non-flag arg, so we handle this manually.
func parseSendArgs(args []string) sendParsed {
	var p sendParsed
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--trace":
			p.trace = true
		case "--thread":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				i++
				p.thread = args[i]
			}
		default:
			p.positional = append(p.positional, args[i])
		}
	}
	return p
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `society — A2A agent orchestration

Usage:
  society <command> [arguments]

Commands:
  onboard [--manual] [--deep] Auto-detect and register agents (--deep: probe SSH/Docker for live agents)
  list                       List all registered agents
  remove <name>              Remove an agent
  ping <name>                Health-check an agent
  run --config <path>        Start an agent from config
  send <name> <message>      Send a message to an agent [--thread <id>]
  export [--output <path>]   Export registry
  import <path-or-url>       Import agents
  discover <url>             Discover agent from A2A endpoint
  mcp                        Start MCP server (stdio)
  daemon start [agents...]   Start all agents in background [--agents <dir>]
  daemon stop                Stop the running daemon
  daemon status              Show daemon status and agents
  daemon run [agents...]     Start all agents in foreground [--agents <dir>]
  skill install              Install Claude Code skills
  skill update               Update installed skills
  update                     Update society to the latest release
  version                    Print the current version

Flags:
  --registry <path>          Registry file (default: ~/.society/registry.json, or SOCIETY_REGISTRY env)`)
}
