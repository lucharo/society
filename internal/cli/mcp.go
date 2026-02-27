package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/luischavesdev/society/internal/client"
	"github.com/luischavesdev/society/internal/mcp"
	"github.com/luischavesdev/society/internal/registry"
)

// MCP starts the MCP server on stdio.
func MCP(registryPath string, in io.Reader, out io.Writer) error {
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	sender := client.New(reg)
	srv := mcp.NewServer(registryPath, reg, sender, in, out)
	return srv.Run(context.Background())
}
