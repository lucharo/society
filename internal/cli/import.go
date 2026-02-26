package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

func Import(registryPath, source string, in io.Reader, out io.Writer) error {
	data, err := loadSource(source)
	if err != nil {
		return err
	}

	var rf models.RegistryFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return fmt.Errorf("parsing import data: %w", err)
	}

	// Validate all cards first
	if err := models.ValidateRegistry(rf.Agents); err != nil {
		return fmt.Errorf("invalid import data: %w", err)
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	r := bufio.NewReader(in)
	resolver := func(local, imported models.AgentCard) registry.MergeAction {
		fmt.Fprintf(out, "\n  Agent %q already exists locally.\n", imported.Name)
		fmt.Fprintf(out, "    Local:    %s\n", local.URL)
		fmt.Fprintf(out, "    Imported: %s\n", imported.URL)

		choice := promptChoice(r, out, "  Overwrite?", []string{"y", "n", "rename"}, "n")
		switch choice {
		case "y":
			return registry.MergeAction{Action: "overwrite"}
		case "rename":
			name := prompt(r, out, "  New name", "")
			return registry.MergeAction{Action: "rename", Name: name}
		default:
			return registry.MergeAction{Action: "skip"}
		}
	}

	result, err := reg.Merge(rf.Agents, resolver)
	if err != nil {
		return err
	}

	if err := reg.Save(); err != nil {
		return err
	}

	var parts []string
	if len(result.Added) > 0 {
		parts = append(parts, fmt.Sprintf("imported %d", len(result.Added)))
	}
	if len(result.Skipped) > 0 {
		parts = append(parts, fmt.Sprintf("skipped %d", len(result.Skipped)))
	}
	if len(result.Replaced) > 0 {
		parts = append(parts, fmt.Sprintf("replaced %d", len(result.Replaced)))
	}
	if len(result.Renamed) > 0 {
		parts = append(parts, fmt.Sprintf("renamed %d", len(result.Renamed)))
	}
	fmt.Fprintf(out, "\n  ✓ %s\n", strings.Join(parts, ", "))
	return nil
}

func loadSource(source string) ([]byte, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(source)
		if err != nil {
			return nil, fmt.Errorf("fetching %s: %w", source, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("fetching %s: status %d", source, resp.StatusCode)
		}
		// Cap at 10MB to prevent abuse from untrusted URLs
		return io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", source, err)
	}
	return data, nil
}
