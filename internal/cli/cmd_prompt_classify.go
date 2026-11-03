package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/joeyvictorino/assay/internal/promptclass"
)

func init() {
	Register("prompt-classify", "classify prompts by category and intent", runPromptClassify)
}

func runPromptClassify(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-classify", stderr)
	prompt := fs.String("prompt", "", "single prompt to classify")
	file := fs.String("file", "", "file with one prompt per line")
	format := fs.String("format", "text", "output format: text or json")
	summary := fs.Bool("summary", false, "show summary statistics only")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prompt-classify: unknown --format %q (text | json)\n", *format)
		return ExitError
	}
	if *prompt == "" && *file == "" {
		fmt.Fprintln(stderr, "assay prompt-classify: --prompt or --file is required")
		fs.Usage()
		return ExitError
	}

	var prompts []string
	if *prompt != "" {
		prompts = append(prompts, *prompt)
	}
	if *file != "" {
		lines, err := readLines(*file)
		if err != nil {
			return fail(stderr, "prompt-classify", err)
		}
		prompts = append(prompts, lines...)
	}

	results := promptclass.ClassifyAll(prompts)

	var output string
	if *summary {
		s := promptclass.Summarize(results)
		switch *format {
		case "json":
			b, err := json.MarshalIndent(s, "", "  ")
			if err != nil {
				return fail(stderr, "prompt-classify", err)
			}
			output = string(b) + "\n"
		default:
			output = promptclass.FormatSummaryText(s)
		}
	} else {
		switch *format {
		case "json":
			b, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return fail(stderr, "prompt-classify", err)
			}
			output = string(b) + "\n"
		default:
			output = promptclass.FormatText(results)
		}
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-classify", err)
	}
	return ExitPass
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, sc.Err()
}
