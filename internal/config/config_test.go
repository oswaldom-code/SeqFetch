package config_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/oswaldom-code/seqfetch/internal/config"
)

var _ = Describe("Load and Save", func() {
	var path string

	BeforeEach(func() {
		path = filepath.Join(GinkgoT().TempDir(), "nested", "config.yaml")
	})

	It("returns a zero Config when the file does not exist", func() {
		cfg, err := config.Load(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).To(Equal(config.Config{}))
	})

	It("round-trips the download dir, creating parent directories", func() {
		Expect(config.Save(path, config.Config{DownloadDir: "/data/dl"})).To(Succeed())
		cfg, err := config.Load(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.DownloadDir).To(Equal("/data/dl"))
	})

	It("fails on malformed YAML", func() {
		Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
		Expect(os.WriteFile(path, []byte("download_dir: [unclosed"), 0o644)).To(Succeed())
		_, err := config.Load(path)
		Expect(err).To(MatchError(ContainSubstring("parse config")))
	})
})

var _ = Describe("ResolveDownloadDir", func() {
	It("prefers the explicit flag over the config", func() {
		dir, err := config.ResolveDownloadDir("/flag", config.Config{DownloadDir: "/cfg"})
		Expect(err).NotTo(HaveOccurred())
		Expect(dir).To(Equal("/flag"))
	})

	It("falls back to the config when no flag is given", func() {
		dir, err := config.ResolveDownloadDir("", config.Config{DownloadDir: "/cfg"})
		Expect(err).NotTo(HaveOccurred())
		Expect(dir).To(Equal("/cfg"))
	})

	It("falls back to the working directory when neither is set", func() {
		cwd, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		dir, err := config.ResolveDownloadDir("", config.Config{})
		Expect(err).NotTo(HaveOccurred())
		Expect(dir).To(Equal(cwd))
	})

	It("makes relative paths absolute", func() {
		cwd, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		dir, err := config.ResolveDownloadDir("out", config.Config{})
		Expect(err).NotTo(HaveOccurred())
		Expect(dir).To(Equal(filepath.Join(cwd, "out")))
	})
})
