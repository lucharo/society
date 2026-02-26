package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/luischavesdev/society/internal/registry"
)

func Export(registryPath string, outputPath string, out io.Writer) error {
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	rf := reg.Export()
	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling registry: %w", err)
	}
	data = append(data, '\n')

	if outputPath == "" {
		_, err = out.Write(data)
		return err
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	fmt.Fprintf(out, "  ✓ Exported to %s\n", outputPath)
	return nil
}
