package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/joeyvictorino/assay/internal/annotate"
)

func init() {
	Register("findings-annotate", "add analyst notes to findings JSON: add | list | summary", runFindingsAnnotate)
}

func runFindingsAnnotate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		findingsAnnotateUsage(stderr)
		return ExitError
	}
	switch args[0] {
	case "add":
		return findingsAnnotateAdd(args[1:], stdout, stderr)
	case "list":
		return findingsAnnotateList(args[1:], stdout, stderr)
	case "summary":
		return findingsAnnotateSummary(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		findingsAnnotateUsage(stdout)
		return ExitPass
	default:
		fmt.Fprintf(stderr, "assay findings-annotate: unknown action %q\n", args[0])
		findingsAnnotateUsage(stderr)
		return ExitError
	}
}

func findingsAnnotateUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  assay findings-annotate add --input FILE --finding-id ID --note TEXT [--author NAME] [--tags t1,t2] [--status STATUS] [--output FILE]")
	fmt.Fprintln(w, "  assay findings-annotate list --input FILE [--finding-id ID] [--format text|json]")
	fmt.Fprintln(w, "  assay findings-annotate summary --input FILE [--format text|json]")
}

func findingsAnnotateAdd(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-annotate add", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	output := fs.String("output", "", "output path (defaults to input)")
	findingID := fs.String("finding-id", "", "ID of finding to annotate (required)")
	note := fs.String("note", "", "annotation text (required)")
	author := fs.String("author", "", "analyst name or email")
	tags := fs.String("tags", "", "comma-separated tags")
	status := fs.String("status", "", "status: confirmed, false_positive, investigating, resolved")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" || *findingID == "" || *note == "" {
		fmt.Fprintln(stderr, "assay findings-annotate add: --input, --finding-id, and --note are required")
		return ExitError
	}
	if *output == "" {
		*output = *input
	}

	af, err := annotate.LoadAnnotated(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-annotate add: %v\n", err)
		return ExitError
	}

	var tagList []string
	if *tags != "" {
		tagList = strings.Split(*tags, ",")
	}

	if err := annotate.AddAnnotation(af, *findingID, *author, *note, tagList, *status); err != nil {
		fmt.Fprintf(stderr, "assay findings-annotate add: %v\n", err)
		return ExitError
	}

	if err := annotate.Save(af, *output); err != nil {
		fmt.Fprintf(stderr, "assay findings-annotate add: %v\n", err)
		return ExitError
	}

	fmt.Fprintf(stdout, "Added annotation to finding %s in %s\n", *findingID, *output)
	return ExitPass
}

func findingsAnnotateList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-annotate list", stderr)
	input := fs.String("input", "", "path to annotated findings JSON (required)")
	findingID := fs.String("finding-id", "", "filter to specific finding")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-annotate list: --input is required")
		return ExitError
	}

	af, err := annotate.LoadAnnotated(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-annotate list: %v\n", err)
		return ExitError
	}

	annotations := annotate.ListAnnotations(af, *findingID)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(annotations)
	case "text":
		fmt.Fprint(stdout, annotate.FormatAnnotationsText(annotations))
	default:
		fmt.Fprintf(stderr, "assay findings-annotate list: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}

func findingsAnnotateSummary(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-annotate summary", stderr)
	input := fs.String("input", "", "path to annotated findings JSON (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-annotate summary: --input is required")
		return ExitError
	}

	af, err := annotate.LoadAnnotated(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-annotate summary: %v\n", err)
		return ExitError
	}

	s := annotate.Summary(af)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(s)
	case "text":
		fmt.Fprintf(stdout, "Findings:    %v\n", s["total_findings"])
		fmt.Fprintf(stdout, "Annotated:   %v\n", s["annotated_findings"])
		fmt.Fprintf(stdout, "Annotations: %v\n", s["total_annotations"])
		if byStatus, ok := s["by_status"].(map[string]int); ok && len(byStatus) > 0 {
			fmt.Fprintln(stdout, "By status:")
			for k, v := range byStatus {
				fmt.Fprintf(stdout, "  %-20s %d\n", k, v)
			}
		}
	default:
		fmt.Fprintf(stderr, "assay findings-annotate summary: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}
