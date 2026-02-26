package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func prompt(r *bufio.Reader, w io.Writer, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Fprintf(w, "  %s (default: %s): ", label, defaultVal)
	} else {
		fmt.Fprintf(w, "  %s: ", label)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func promptChoice(r *bufio.Reader, w io.Writer, label string, options []string, defaultVal string) string {
	optStr := strings.Join(options, "/")
	if defaultVal != "" {
		fmt.Fprintf(w, "  %s [%s] (default: %s): ", label, optStr, defaultVal)
	} else {
		fmt.Fprintf(w, "  %s [%s]: ", label, optStr)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	for _, o := range options {
		if strings.EqualFold(line, o) {
			return o
		}
	}
	return line
}

func promptYN(r *bufio.Reader, w io.Writer, label string, defaultYes bool) bool {
	if defaultYes {
		fmt.Fprintf(w, "  %s [Y/n]: ", label)
	} else {
		fmt.Fprintf(w, "  %s [y/N]: ", label)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}
