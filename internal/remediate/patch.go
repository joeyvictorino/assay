package remediate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// A minimal unified-diff applier so Verify needs no external tools. It
// supports the subset git and diff -u emit for text files: ---/+++ headers
// with a/ and b/ prefixes, /dev/null for created or deleted files, and @@
// hunks whose context is matched exactly (at the stated line first, then by
// searching the file). Binary patches, renames and mode changes are rejected.

type hunk struct {
	oldStart, newStart int
	lines              []string // each starts with ' ', '-' or '+'
}

type filePatch struct {
	oldPath, newPath string
	hunks            []hunk
}

// ApplyUnified applies diff to the files under root.
func ApplyUnified(root, diff string) error {
	patches, err := parseUnified(diff)
	if err != nil {
		return err
	}
	if len(patches) == 0 {
		return fmt.Errorf("patch: no file hunks found")
	}
	for _, p := range patches {
		if err := applyFile(root, p); err != nil {
			return err
		}
	}
	return nil
}

func parseUnified(diff string) ([]filePatch, error) {
	lines := strings.Split(strings.ReplaceAll(diff, "\r\n", "\n"), "\n")
	var out []filePatch
	var cur *filePatch
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch {
		case strings.HasPrefix(l, "--- "):
			if i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "+++ ") {
				return nil, fmt.Errorf("patch: line %d: --- without +++", i+1)
			}
			out = append(out, filePatch{oldPath: cleanPath(l[4:]), newPath: cleanPath(lines[i+1][4:])})
			cur = &out[len(out)-1]
			i++
		case strings.HasPrefix(l, "@@ "):
			if cur == nil {
				return nil, fmt.Errorf("patch: line %d: hunk before file header", i+1)
			}
			h, oldN, newN, err := parseHunkHeader(l)
			if err != nil {
				return nil, fmt.Errorf("patch: line %d: %w", i+1, err)
			}
			// Consume exactly the counted lines so a following "--- " file
			// header is never mistaken for a removed line.
			for (oldN > 0 || newN > 0) && i+1 < len(lines) {
				n := lines[i+1]
				if strings.HasPrefix(n, "\\ No newline") {
					i++
					continue
				}
				if n == "" {
					n = " " // some tools emit an empty line for an empty context line
				}
				switch n[0] {
				case ' ':
					oldN--
					newN--
				case '-':
					oldN--
				case '+':
					newN--
				default:
					return nil, fmt.Errorf("patch: line %d: unexpected %q inside hunk", i+2, n)
				}
				h.lines = append(h.lines, n)
				i++
			}
			if oldN != 0 || newN != 0 {
				return nil, fmt.Errorf("patch: hunk at line %d is truncated", i+1)
			}
			cur.hunks = append(cur.hunks, h)
		case strings.HasPrefix(l, "GIT binary patch"), strings.HasPrefix(l, "Binary files"):
			return nil, fmt.Errorf("patch: binary patches are not supported")
		case strings.HasPrefix(l, "rename from"), strings.HasPrefix(l, "old mode"):
			return nil, fmt.Errorf("patch: renames and mode changes are not supported")
		}
	}
	return out, nil
}

func cleanPath(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\t'); i >= 0 {
		s = s[:i]
	}
	if s == "/dev/null" {
		return s
	}
	s = strings.TrimPrefix(s, "a/")
	s = strings.TrimPrefix(s, "b/")
	return s
}

// parseHunkHeader reads "@@ -l[,s] +l[,s] @@ [section]" and returns the
// hunk plus the old and new line counts (default 1 when omitted).
func parseHunkHeader(l string) (hunk, int, int, error) {
	parts := strings.Fields(l)
	if len(parts) < 3 || !strings.HasPrefix(parts[1], "-") || !strings.HasPrefix(parts[2], "+") {
		return hunk{}, 0, 0, fmt.Errorf("bad hunk header %q", l)
	}
	o, oc, err := parseRange(parts[1][1:])
	if err != nil {
		return hunk{}, 0, 0, fmt.Errorf("bad hunk header %q", l)
	}
	n, nc, err := parseRange(parts[2][1:])
	if err != nil {
		return hunk{}, 0, 0, fmt.Errorf("bad hunk header %q", l)
	}
	return hunk{oldStart: o, newStart: n}, oc, nc, nil
}

func parseRange(s string) (start, count int, err error) {
	a, b, hasCount := strings.Cut(s, ",")
	if start, err = strconv.Atoi(a); err != nil {
		return 0, 0, err
	}
	count = 1
	if hasCount {
		if count, err = strconv.Atoi(b); err != nil {
			return 0, 0, err
		}
	}
	return start, count, nil
}

func safeJoin(root, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("patch: refusing path %q", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("patch: refusing path %q", rel)
	}
	return filepath.Join(root, clean), nil
}

func applyFile(root string, p filePatch) error {
	// Deletion.
	if p.newPath == "/dev/null" {
		path, err := safeJoin(root, p.oldPath)
		if err != nil {
			return err
		}
		return os.Remove(path)
	}
	path, err := safeJoin(root, p.newPath)
	if err != nil {
		return err
	}
	var content []string
	hadTrailingNewline := true
	if p.oldPath != "/dev/null" {
		src, err := safeJoin(root, p.oldPath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("patch: %w", err)
		}
		text := strings.ReplaceAll(string(data), "\r\n", "\n")
		hadTrailingNewline = strings.HasSuffix(text, "\n")
		text = strings.TrimSuffix(text, "\n")
		if text != "" {
			content = strings.Split(text, "\n")
		}
	}
	offset := 0
	for hi, h := range p.hunks {
		var oldLines, newLines []string
		for _, l := range h.lines {
			switch l[0] {
			case ' ':
				oldLines = append(oldLines, l[1:])
				newLines = append(newLines, l[1:])
			case '-':
				oldLines = append(oldLines, l[1:])
			case '+':
				newLines = append(newLines, l[1:])
			}
		}
		pos := -1
		want := h.oldStart - 1 + offset
		if len(oldLines) == 0 {
			pos = want
			if pos < 0 {
				pos = 0
			}
			if pos > len(content) {
				pos = len(content)
			}
		} else {
			if matchesAt(content, want, oldLines) {
				pos = want
			} else {
				for i := 0; i+len(oldLines) <= len(content); i++ {
					if matchesAt(content, i, oldLines) {
						pos = i
						break
					}
				}
			}
		}
		if pos < 0 {
			return fmt.Errorf("patch: %s: hunk %d does not apply", p.newPath, hi+1)
		}
		next := make([]string, 0, len(content)-len(oldLines)+len(newLines))
		next = append(next, content[:pos]...)
		next = append(next, newLines...)
		next = append(next, content[pos+len(oldLines):]...)
		content = next
		offset += (pos - want) + len(newLines) - len(oldLines)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("patch: %w", err)
	}
	out := strings.Join(content, "\n")
	if hadTrailingNewline || p.oldPath == "/dev/null" {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

func matchesAt(content []string, pos int, want []string) bool {
	if pos < 0 || pos+len(want) > len(content) {
		return false
	}
	for i, w := range want {
		if content[pos+i] != w {
			return false
		}
	}
	return true
}
