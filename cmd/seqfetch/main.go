// Command seqfetch downloads sequentially named files from a URL template.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "seqfetch",
		Version: version,
		Short:   "Download sequentially named files from a URL template",
		Long: `seqfetch downloads files whose names follow a numeric sequence.

The URL template must contain exactly one placeholder for the sequence:

  {n}     the index as-is           https://host/docs/{n}.pdf   -> 7.pdf
  {n:04}  zero-padded to 4 digits   https://host/img/IMG_{n:04}.jpg -> IMG_0007.jpg

Downloading stops at the first index that returns HTTP 404 or 403. With
--backward it first walks down from --start to find the beginning of the
sequence. Files already present in the download directory are skipped, so
re-running resumes.`,
		SilenceUsage: true,
	}
	root.AddCommand(newFetchCmd(), newConfigCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
