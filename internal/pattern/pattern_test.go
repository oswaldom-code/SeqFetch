package pattern_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/oswaldom-code/seqfetch/internal/pattern"
)

var _ = Describe("Parse", func() {
	It("renders a numeric-only name without padding", func() {
		p, err := pattern.Parse("https://host.com/docs/{n}.pdf")
		Expect(err).NotTo(HaveOccurred())
		Expect(p.URL(7)).To(Equal("https://host.com/docs/7.pdf"))
		Expect(p.URL(123)).To(Equal("https://host.com/docs/123.pdf"))
	})

	It("renders a static prefix with zero padding", func() {
		p, err := pattern.Parse("https://host.com/img/IMG_{n:04}.jpg")
		Expect(err).NotTo(HaveOccurred())
		Expect(p.URL(7)).To(Equal("https://host.com/img/IMG_0007.jpg"))
		Expect(p.URL(12345)).To(Equal("https://host.com/img/IMG_12345.jpg"))
	})

	It("derives the file name from the rendered URL path, ignoring the query", func() {
		p, err := pattern.Parse("https://host.com/a/b/part-{n:02}.zip?token=abc")
		Expect(err).NotTo(HaveOccurred())
		Expect(p.FileName(3)).To(Equal("part-03.zip"))
	})

	DescribeTable("prefixes the file name with the index when the placeholder is not in the last segment",
		func(template string, n int, want string) {
			p, err := pattern.Parse(template)
			Expect(err).NotTo(HaveOccurred())
			Expect(p.FileName(n)).To(Equal(want))
		},
		Entry("middle path segment", "https://picsum.photos/id/{n}/200/300", 10, "10_300"),
		Entry("middle segment with padding", "https://host.com/item/{n:03}/file.bin", 7, "007_file.bin"),
		Entry("query string", "https://host.com/download.pdf?id={n}", 7, "7_download.pdf"),
		Entry("host name", "https://cdn{n}.host.com/asset.js", 2, "2_asset.js"),
	)

	DescribeTable("rejects invalid templates",
		func(template string, msg string) {
			_, err := pattern.Parse(template)
			Expect(err).To(MatchError(ContainSubstring(msg)))
		},
		Entry("no placeholder", "https://host.com/file.pdf", "placeholder"),
		Entry("two placeholders", "https://host.com/{n}/{n}.pdf", "exactly one"),
		Entry("zero width", "https://host.com/{n:0}.pdf", "padding width"),
		Entry("unsupported scheme", "ftp://host.com/{n}.pdf", "http or https"),
		Entry("missing host", "https:///{n}.pdf", "host"),
		Entry("no file name", "https://host.com/{n}/", "file name"),
	)
})
