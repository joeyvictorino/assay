package report

import (
	"fmt"
	"html"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/reconcile"
)

// HTML wraps the Markdown report in a self-contained HTML page with
// inline styles. The Markdown is included in a <pre> section with
// basic table rendering converted to HTML tables. This keeps the
// output dependency-free (no JS markdown renderer needed).
func HTML(rr model.RunReport, findings []model.Finding, rec *reconcile.Report) string {
	md := Markdown(rr, findings, rec)
	body := markdownToHTML(md)

	var b strings.Builder
	w := func(s string) { b.WriteString(s) }

	w("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	w("<meta charset=\"utf-8\">\n")
	w("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	w(fmt.Sprintf("<title>assay run %s</title>\n", html.EscapeString(orDash(rr.RunID))))
	w("<style>\n")
	w(`:root { --bg: #fff; --fg: #1a1a2e; --muted: #6b7280; --border: #e5e7eb; --accent: #2563eb; --code-bg: #f3f4f6; --table-stripe: #f9fafb; }
@media (prefers-color-scheme: dark) {
  :root { --bg: #0f172a; --fg: #e2e8f0; --muted: #94a3b8; --border: #334155; --accent: #60a5fa; --code-bg: #1e293b; --table-stripe: #1e293b; }
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; color: var(--fg); background: var(--bg); max-width: 900px; margin: 0 auto; padding: 2rem 1rem; line-height: 1.6; }
h1 { font-size: 1.5rem; margin: 1.5rem 0 0.75rem; border-bottom: 2px solid var(--accent); padding-bottom: 0.25rem; }
h2 { font-size: 1.25rem; margin: 1.25rem 0 0.5rem; color: var(--accent); }
h3 { font-size: 1.1rem; margin: 1rem 0 0.5rem; }
p { margin: 0.5rem 0; }
ul, ol { margin: 0.5rem 0 0.5rem 1.5rem; }
li { margin: 0.25rem 0; }
code { font-family: "SF Mono", "Fira Code", monospace; font-size: 0.875em; background: var(--code-bg); padding: 0.125rem 0.375rem; border-radius: 3px; }
pre { background: var(--code-bg); padding: 1rem; border-radius: 6px; overflow-x: auto; margin: 0.75rem 0; font-size: 0.85rem; }
pre code { background: none; padding: 0; }
blockquote { border-left: 3px solid var(--accent); padding: 0.5rem 1rem; margin: 0.5rem 0; color: var(--muted); }
table { width: 100%%; border-collapse: collapse; margin: 0.75rem 0; font-size: 0.9rem; }
th { background: var(--code-bg); text-align: left; padding: 0.5rem 0.75rem; border: 1px solid var(--border); font-weight: 600; }
td { padding: 0.5rem 0.75rem; border: 1px solid var(--border); }
tr:nth-child(even) td { background: var(--table-stripe); }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
`)
	w("</style>\n</head>\n<body>\n")
	w(body)
	w("\n</body>\n</html>\n")
	return b.String()
}

// markdownToHTML does a lightweight conversion of the Markdown report
// to HTML. It handles headings, tables, lists, code blocks, blockquotes,
// inline code, bold, and links. This avoids pulling in a full markdown
// parser as a dependency.
func markdownToHTML(md string) string {
	var out strings.Builder
	lines := strings.Split(md, "\n")
	inTable := false
	inList := false
	inCode := false
	inBlockquote := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Fenced code blocks.
		if strings.HasPrefix(line, "```") {
			if inCode {
				out.WriteString("</code></pre>\n")
				inCode = false
			} else {
				closeBlock(&out, &inTable, &inList, &inBlockquote)
				out.WriteString("<pre><code>")
				inCode = true
			}
			continue
		}
		if inCode {
			out.WriteString(html.EscapeString(line) + "\n")
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			closeBlock(&out, &inTable, &inList, &inBlockquote)
			continue
		}

		// Headings.
		if strings.HasPrefix(trimmed, "#") {
			closeBlock(&out, &inTable, &inList, &inBlockquote)
			level := 0
			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			text := strings.TrimSpace(trimmed[level:])
			out.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, inlineFormat(text), level))
			continue
		}

		// Blockquote.
		if strings.HasPrefix(trimmed, "> ") {
			if !inBlockquote {
				closeBlock(&out, &inTable, &inList, &inBlockquote)
				out.WriteString("<blockquote>\n")
				inBlockquote = true
			}
			out.WriteString("<p>" + inlineFormat(strings.TrimPrefix(trimmed, "> ")) + "</p>\n")
			continue
		}

		// Table rows.
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			cells := parseTableRow(trimmed)
			if isAlignRow(trimmed) {
				continue
			}
			if !inTable {
				closeBlock(&out, &inTable, &inList, &inBlockquote)
				out.WriteString("<table>\n<thead><tr>")
				for _, c := range cells {
					out.WriteString("<th>" + inlineFormat(c) + "</th>")
				}
				out.WriteString("</tr></thead>\n<tbody>\n")
				inTable = true
			} else {
				out.WriteString("<tr>")
				for _, c := range cells {
					out.WriteString("<td>" + inlineFormat(c) + "</td>")
				}
				out.WriteString("</tr>\n")
			}
			continue
		}

		// Unordered list.
		if strings.HasPrefix(trimmed, "- ") {
			if !inList {
				closeBlock(&out, &inTable, &inList, &inBlockquote)
				out.WriteString("<ul>\n")
				inList = true
			}
			out.WriteString("<li>" + inlineFormat(strings.TrimPrefix(trimmed, "- ")) + "</li>\n")
			continue
		}

		// Paragraph.
		if inList || inTable || inBlockquote {
			closeBlock(&out, &inTable, &inList, &inBlockquote)
		}
		out.WriteString("<p>" + inlineFormat(trimmed) + "</p>\n")
	}

	closeBlock(&out, &inTable, &inList, &inBlockquote)
	if inCode {
		out.WriteString("</code></pre>\n")
	}
	return out.String()
}

func closeBlock(out *strings.Builder, inTable, inList, inBlockquote *bool) {
	if *inTable {
		out.WriteString("</tbody></table>\n")
		*inTable = false
	}
	if *inList {
		out.WriteString("</ul>\n")
		*inList = false
	}
	if *inBlockquote {
		out.WriteString("</blockquote>\n")
		*inBlockquote = false
	}
}

func parseTableRow(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	parts := strings.Split(row, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func isAlignRow(row string) bool {
	cells := parseTableRow(row)
	for _, c := range cells {
		cleaned := strings.Trim(c, ":-")
		if cleaned != "" {
			return false
		}
	}
	return true
}

// inlineFormat handles bold, code, and links.
func inlineFormat(s string) string {
	s = html.EscapeString(s)
	// Inline code: `...`
	s = replaceInlineCode(s)
	// Bold: **...**
	s = replaceBold(s)
	// Links: <url>
	s = replaceAngleLinks(s)
	return s
}

func replaceInlineCode(s string) string {
	var out strings.Builder
	for {
		start := strings.Index(s, "`")
		if start < 0 {
			out.WriteString(s)
			break
		}
		end := strings.Index(s[start+1:], "`")
		if end < 0 {
			out.WriteString(s)
			break
		}
		end += start + 1
		out.WriteString(s[:start])
		out.WriteString("<code>" + s[start+1:end] + "</code>")
		s = s[end+1:]
	}
	return out.String()
}

func replaceBold(s string) string {
	var out strings.Builder
	for {
		start := strings.Index(s, "**")
		if start < 0 {
			out.WriteString(s)
			break
		}
		end := strings.Index(s[start+2:], "**")
		if end < 0 {
			out.WriteString(s)
			break
		}
		end += start + 2
		out.WriteString(s[:start])
		out.WriteString("<strong>" + s[start+2:end] + "</strong>")
		s = s[end+2:]
	}
	return out.String()
}

func replaceAngleLinks(s string) string {
	var out strings.Builder
	for {
		start := strings.Index(s, "&lt;http")
		if start < 0 {
			out.WriteString(s)
			break
		}
		end := strings.Index(s[start:], "&gt;")
		if end < 0 {
			out.WriteString(s)
			break
		}
		end += start
		url := strings.TrimPrefix(strings.TrimSuffix(s[start:end+4], "&gt;"), "&lt;")
		out.WriteString(s[:start])
		out.WriteString(fmt.Sprintf(`<a href="%s">%s</a>`, url, url))
		s = s[end+4:]
	}
	return out.String()
}
