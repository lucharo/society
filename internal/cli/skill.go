package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SkillInstall installs society skills for Claude Code from the given embedded FS.
func SkillInstall(skillsFS fs.FS, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	claudeDir := filepath.Join(home, ".claude")
	if _, err := os.Stat(claudeDir); err != nil {
		return fmt.Errorf("~/.claude not found — is Claude Code installed?")
	}

	skillsDir := filepath.Join(claudeDir, "skills")

	entries, err := fs.ReadDir(skillsFS, ".")
	if err != nil {
		return fmt.Errorf("reading embedded skills: %w", err)
	}

	installed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := fs.ReadFile(skillsFS, entry.Name())
		if err != nil {
			fmt.Fprintf(out, "  warning: could not read %s: %v\n", entry.Name(), err)
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		dir := filepath.Join(skillsDir, "society-"+name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating skill directory: %w", err)
		}

		dest := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("writing skill file: %w", err)
		}
		fmt.Fprintf(out, "  installed society:%s\n", name)
		installed++
	}

	if installed == 0 {
		fmt.Fprintln(out, "  no skills to install")
	} else {
		fmt.Fprintf(out, "\n  %d skill(s) installed for Claude Code\n", installed)
	}

	return nil
}
