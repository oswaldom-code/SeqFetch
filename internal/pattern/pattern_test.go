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
