package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/oswaldom-code/seqfetch/internal/config"
)

const keyDownloadDir = "download-dir"

func runConfigSet(key, value string) error {
	if key != keyDownloadDir {
		return fmt.Errorf("unknown key %q (supported: %s)", key, keyDownloadDir)
	}
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	cfg.DownloadDir, err = filepath.Abs(value)
	if err != nil {
		return err
	}
	return config.Save(path, cfg)
}

func runConfigShow(cmd *cobra.Command) error {
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	dir := cfg.DownloadDir
	if dir == "" {
		dir = "(not set, using current directory)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "config file:  %s\n%s: %s\n", path, keyDownloadDir, dir)
	return nil
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage persistent settings",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:     "set <key> <value>",
			Short:   "Persist a setting",
			Example: "  seqfetch config set download-dir ~/Downloads/seq",
			Args:    cobra.ExactArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				return runConfigSet(args[0], args[1])
			},
		},
		&cobra.Command{
			Use:   "show",
			Short: "Print the current settings",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return runConfigShow(cmd)
			},
		},
	)
	return cmd
}
