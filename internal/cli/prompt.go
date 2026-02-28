package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ANSI escape codes
const (
	bold  = "\033[1m"
	dim   = "\033[2m"
	red   = "\033[31m"
	green = "\033[32m"
	cyan  = "\033[36m"
	reset = "\033[0m"
)

func prompt(r *bufio.Reader, w io.Writer, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Fprintf(w, "%s%s%s %s(default: %s)%s: ", bold, label, reset, dim, defaultVal, reset)
	} else {
		fmt.Fprintf(w, "%s%s%s: ", bold, label, reset)
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
		fmt.Fprintf(w, "%s%s%s [%s] %s(default: %s)%s: ", bold, label, reset, optStr, dim, defaultVal, reset)
	} else {
		fmt.Fprintf(w, "%s%s%s [%s]: ", bold, label, reset, optStr)
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
		fmt.Fprintf(w, "%s%s%s [Y/n]: ", bold, label, reset)
	} else {
		fmt.Fprintf(w, "%s%s%s [y/N]: ", bold, label, reset)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}
