package cli

import (
	"bufio"
	"fmt"
	"io"

	"github.com/luischavesdev/society/internal/registry"
)

func Remove(registryPath, name string, in io.Reader, out io.Writer) error {
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	if !reg.Has(name) {
		return fmt.Errorf("agent %q not found", name)
	}

	r := bufio.NewReader(in)
	if !promptYN(r, out, fmt.Sprintf("Remove agent %q?", name), false) {
		fmt.Fprintln(out, "  Cancelled")
		return nil
	}

	if err := reg.Remove(name); err != nil {
		return err
	}
	if err := reg.Save(); err != nil {
		return err
	}

	fmt.Fprintf(out, "  ✓ Removed %q\n", name)
	return nil
}
