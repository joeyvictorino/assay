package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/promptbank"
)

func init() {
	Register("prompt-bank", "manage prompt templates for assessment scenarios", runPromptBank)
}

func runPromptBank(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "assay prompt-bank: subcommand required (list | show | add | export)")
		return ExitError
	}
	sub := args[0]
	rest := args[1:]

	switch sub {
	case "list":
		return runPromptBankList(rest, stdout, stderr)
	case "show":
		return runPromptBankShow(rest, stdout, stderr)
	case "add":
		return runPromptBankAdd(rest, stdout, stderr)
	case "export":
		return runPromptBankExport(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "assay prompt-bank: unknown subcommand %q\n", sub)
		return ExitError
	}
}

func bankDir() string {
	return filepath.Join(".assay", "prompts")
}

func runPromptBankList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-bank list", stderr)
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	b := promptbank.New(bankDir())
	templates, err := b.List()
	if err != nil {
		return fail(stderr, "prompt-bank list", err)
	}

	switch *format {
	case "json":
		data, _ := json.MarshalIndent(templates, "", "  ")
		fmt.Fprintln(stdout, string(data))
	default:
		if len(templates) == 0 {
			fmt.Fprintln(stdout, "No prompt templates found.")
			return ExitPass
		}
		fmt.Fprintf(stdout, "Prompt Templates (%d)\n\n", len(templates))
		for _, t := range templates {
			fmt.Fprintf(stdout, "  %-20s [%s] %s\n", t.Name, t.Category, t.Description)
		}
	}
	return ExitPass
}

func runPromptBankShow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-bank show", stderr)
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "assay prompt-bank show: template name required")
		return ExitError
	}

	name := fs.Arg(0)
	b := promptbank.New(bankDir())
	t, err := b.Get(name)
	if err != nil {
		return fail(stderr, "prompt-bank show", err)
	}

	switch *format {
	case "json":
		data, _ := json.MarshalIndent(t, "", "  ")
		fmt.Fprintln(stdout, string(data))
	default:
		fmt.Fprintf(stdout, "Name:        %s\n", t.Name)
		fmt.Fprintf(stdout, "Category:    %s\n", t.Category)
		fmt.Fprintf(stdout, "Description: %s\n\n", t.Description)
		fmt.Fprintf(stdout, "System Prompt:\n  %s\n\n", t.SystemPrompt)
		fmt.Fprintf(stdout, "User Prompt Template:\n  %s\n", t.UserPromptTemplate)
		if len(t.Variables) > 0 {
			fmt.Fprintf(stdout, "\nVariables:\n")
			for _, v := range t.Variables {
				def := ""
				if v.Default != "" {
					def = fmt.Sprintf(" (default: %s)", v.Default)
				}
				fmt.Fprintf(stdout, "  %-15s %s%s\n", v.Name, v.Description, def)
			}
		}
	}
	return ExitPass
}

func runPromptBankAdd(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-bank add", stderr)
	file := fs.String("file", "", "path to template JSON file (required)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *file == "" {
		fmt.Fprintln(stderr, "assay prompt-bank add: --file is required")
		return ExitError
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		return fail(stderr, "prompt-bank add", err)
	}
	var t promptbank.Template
	if err := json.Unmarshal(data, &t); err != nil {
		return fail(stderr, "prompt-bank add", fmt.Errorf("parse %s: %w", *file, err))
	}

	b := promptbank.New(bankDir())
	if err := b.Add(t); err != nil {
		return fail(stderr, "prompt-bank add", err)
	}
	fmt.Fprintf(stdout, "Added template %q to %s\n", t.Name, b.Dir())
	return ExitPass
}

func runPromptBankExport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-bank export", stderr)
	format := fs.String("format", "json", "output format: json or yaml")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	b := promptbank.New(bankDir())
	out, err := b.Export(*format)
	if err != nil {
		return fail(stderr, "prompt-bank export", err)
	}
	fmt.Fprint(stdout, out)
	return ExitPass
}
