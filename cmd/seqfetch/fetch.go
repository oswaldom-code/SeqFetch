package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/oswaldom-code/seqfetch/internal/config"
	"github.com/oswaldom-code/seqfetch/internal/downloader"
	"github.com/oswaldom-code/seqfetch/internal/pattern"
)

func runFetch(cmd *cobra.Command, template string, start, workers, limit int, outFlag string, backward bool) error {
	p, err := pattern.Parse(template)
	if err != nil {
		return err
	}

	cfgPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	outDir, err := config.ResolveDownloadDir(outFlag, cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create download dir: %w", err)
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading to %s (workers=%d, start=%d, backward=%t, limit=%d)\n",
		outDir, workers, start, backward, limit)
	d := &downloader.Downloader{Workers: workers, OutDir: outDir, Out: cmd.OutOrStdout(), Backward: backward, Limit: limit}
	s, runErr := d.Run(ctx, p, start)

	fmt.Fprintf(cmd.OutOrStdout(), "\nDownloaded %d file(s), skipped %d existing, range %d..%d, %d failure(s)\n",
		s.Downloaded, s.Skipped, s.FirstIndex, s.StoppedAt-1, len(s.Failures))
	if runErr != nil {
		return runErr
	}
	if len(s.Failures) > 0 {
		return fmt.Errorf("%d download(s) failed", len(s.Failures))
	}
	return nil
}

func newFetchCmd() *cobra.Command {
	var (
		start    int
		workers  int
		limit    int
		outFlag  string
		backward bool
	)
	cmd := &cobra.Command{
		Use:   "fetch <url-template>",
		Short: "Download files from a URL template until the first 404/403",
		Example: `  seqfetch fetch "https://host.com/docs/{n}.pdf"
  seqfetch fetch "https://host.com/img/IMG_{n:04}.jpg" --start 100 --workers 8 --out ~/Pictures
  seqfetch fetch "https://host.com/docs/{n}.pdf" --start 123 --backward
  seqfetch fetch "https://host.com/docs/{n}.pdf" --limit 2   # try the template on two files`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetch(cmd, args[0], start, workers, limit, outFlag, backward)
		},
	}
	cmd.Flags().IntVar(&start, "start", 1, "first index of the sequence")
	cmd.Flags().IntVar(&workers, "workers", 4, "number of concurrent downloads")
	cmd.Flags().StringVar(&outFlag, "out", "", "download directory (overrides the configured one)")
	cmd.Flags().BoolVar(&backward, "backward", false, "also walk down from --start until the first missing index")
	cmd.Flags().IntVar(&limit, "limit", 0, "process at most N indices in total, 0 means unlimited (useful to test a template)")
	return cmd
}
