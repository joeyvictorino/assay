package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/version"
)

func init() {
	Register("version", "print build version", func(_ []string, stdout, _ io.Writer) int {
		fmt.Fprintf(stdout, "assay %s (%s, %s)\n", version.Version, version.Commit, version.Date)
		return ExitPass
	})
}
