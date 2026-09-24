package downloader_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/oswaldom-code/seqfetch/internal/downloader"
	"github.com/oswaldom-code/seqfetch/internal/pattern"
)

// hitLog records the indices requested, in order.
type hitLog struct {
	mu sync.Mutex
	ns []int
}

func (h *hitLog) add(n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ns = append(h.ns, n)
}

func (h *hitLog) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.ns)
}

// newServer serves /f/<n>.txt with body "file <n>" for 1 <= n <= last.
// Indices in overrides answer with the given status instead.
func newServer(last int, overrides map[int]int) (*httptest.Server, *hitLog) {
	hits := &hitLog{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/f/"), ".txt")
		n, err := strconv.Atoi(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		hits.add(n)
		if code, ok := overrides[n]; ok {
			http.Error(w, http.StatusText(code), code)
			return
		}
		if n < 1 || n > last {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, "file %d", n)
	})
	return httptest.NewServer(h), hits
}

var _ = Describe("Run", func() {
	var (
		srv    *httptest.Server
		outDir string
		out    bytes.Buffer
	)

	AfterEach(func() {
		srv.Close()
	})

	BeforeEach(func() {
		outDir = GinkgoT().TempDir()
		out.Reset()
	})

	expectFiles := func(from, to int) {
		GinkgoHelper()
		for n := from; n <= to; n++ {
			data, err := os.ReadFile(filepath.Join(outDir, fmt.Sprintf("%d.txt", n)))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(fmt.Sprintf("file %d", n)))
		}
	}

	It("downloads every file until the first 404 using several workers", func() {
		srv, _ = newServer(5, nil)
		p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
		Expect(err).NotTo(HaveOccurred())

		d := &downloader.Downloader{Workers: 4, OutDir: outDir, Out: &out}
		s, err := d.Run(context.Background(), p, 1)
		Expect(err).NotTo(HaveOccurred())

		Expect(s.Downloaded).To(Equal(5))
		Expect(s.FirstIndex).To(Equal(1))
		Expect(s.StoppedAt).To(Equal(6))
		Expect(s.Failures).To(BeEmpty())
		expectFiles(1, 5)
		Expect(out.String()).To(ContainSubstring("OK    3.txt"))
		Expect(out.String()).To(ContainSubstring("MISS  " + srv.URL + "/f/6.txt"))
	})

	It("honours the start index", func() {
		srv, hits := newServer(5, nil)
		p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
		Expect(err).NotTo(HaveOccurred())

		d := &downloader.Downloader{Workers: 1, OutDir: outDir}
		s, err := d.Run(context.Background(), p, 4)
		Expect(err).NotTo(HaveOccurred())

		Expect(s.Downloaded).To(Equal(2))
		Expect(s.StoppedAt).To(Equal(6))
		Expect(hits.count()).To(Equal(3)) // 4, 5, 6
		Expect(filepath.Join(outDir, "3.txt")).NotTo(BeAnExistingFile())
	})

	It("treats 403 as the end of the sequence", func() {
		srv, _ = newServer(5, map[int]int{3: http.StatusForbidden})
		p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
		Expect(err).NotTo(HaveOccurred())

		d := &downloader.Downloader{Workers: 1, OutDir: outDir}
		s, err := d.Run(context.Background(), p, 1)
		Expect(err).NotTo(HaveOccurred())

		Expect(s.Downloaded).To(Equal(2))
		Expect(s.StoppedAt).To(Equal(3))
		Expect(s.Failures).To(BeEmpty())
	})

	It("skips files that already exist without requesting them", func() {
		srv, hits := newServer(5, nil)
		p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
		Expect(err).NotTo(HaveOccurred())
		existing := filepath.Join(outDir, "2.txt")
		Expect(os.WriteFile(existing, []byte("local copy"), 0o644)).To(Succeed())

		d := &downloader.Downloader{Workers: 1, OutDir: outDir, Out: &out}
		s, err := d.Run(context.Background(), p, 1)
		Expect(err).NotTo(HaveOccurred())

		Expect(s.Downloaded).To(Equal(4))
		Expect(s.Skipped).To(Equal(1))
		Expect(s.StoppedAt).To(Equal(6))
		Expect(hits.count()).To(Equal(5)) // 1, 3, 4, 5, 6
		data, err := os.ReadFile(existing)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("local copy"))
		Expect(out.String()).To(ContainSubstring("SKIP  2.txt"))
	})

	It("records non-404 errors and keeps going", func() {
		srv, _ = newServer(4, map[int]int{2: http.StatusInternalServerError})
		p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
		Expect(err).NotTo(HaveOccurred())

		d := &downloader.Downloader{Workers: 2, OutDir: outDir}
		s, err := d.Run(context.Background(), p, 1)
		Expect(err).NotTo(HaveOccurred())

		Expect(s.Downloaded).To(Equal(3))
		Expect(s.StoppedAt).To(Equal(5))
		Expect(s.Failures).To(HaveLen(1))
		Expect(s.Failures[0]).To(MatchError(ContainSubstring("2.txt: unexpected status 500")))
		Expect(filepath.Join(outDir, "2.txt")).NotTo(BeAnExistingFile())
	})

	It("stops early when the context is cancelled", func() {
		srv, _ = newServer(1_000_000, nil)
		p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			Eventually(func() int {
				entries, _ := os.ReadDir(outDir)
				return len(entries)
			}).Should(BeNumerically(">=", 10))
			cancel()
		}()

		d := &downloader.Downloader{Workers: 2, OutDir: outDir}
		_, err = d.Run(ctx, p, 1)
		Expect(err).To(MatchError(context.Canceled))
	})

	Describe("with Limit", func() {
		It("processes at most Limit indices going forward", func() {
			srv, hits := newServer(5, nil)
			p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
			Expect(err).NotTo(HaveOccurred())

			d := &downloader.Downloader{Workers: 4, OutDir: outDir, Limit: 2}
			s, err := d.Run(context.Background(), p, 1)
			Expect(err).NotTo(HaveOccurred())

			Expect(s.Downloaded).To(Equal(2))
			Expect(s.FirstIndex).To(Equal(1))
			Expect(s.StoppedAt).To(Equal(3))
			Expect(hits.count()).To(Equal(2))
			expectFiles(1, 2)
		})

		It("still stops at a miss before the limit is reached", func() {
			srv, hits := newServer(2, nil)
			p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
			Expect(err).NotTo(HaveOccurred())

			d := &downloader.Downloader{Workers: 1, OutDir: outDir, Limit: 10}
			s, err := d.Run(context.Background(), p, 1)
			Expect(err).NotTo(HaveOccurred())

			Expect(s.Downloaded).To(Equal(2))
			Expect(s.StoppedAt).To(Equal(3))
			Expect(hits.ns).To(Equal([]int{1, 2, 3}))
		})

		It("shares the budget between the backward walk and the forward run", func() {
			srv, hits := newServer(5, nil)
			p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
			Expect(err).NotTo(HaveOccurred())

			d := &downloader.Downloader{Workers: 1, OutDir: outDir, Backward: true, Limit: 5}
			s, err := d.Run(context.Background(), p, 4)
			Expect(err).NotTo(HaveOccurred())

			Expect(s.Downloaded).To(Equal(4))
			Expect(s.FirstIndex).To(Equal(1))
			Expect(s.StoppedAt).To(Equal(5))
			Expect(hits.ns).To(Equal([]int{3, 2, 1, 0, 4}))
		})

		It("skips the forward run when the backward walk uses the whole budget", func() {
			srv, hits := newServer(5, nil)
			p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
			Expect(err).NotTo(HaveOccurred())

			d := &downloader.Downloader{Workers: 1, OutDir: outDir, Backward: true, Limit: 2}
			s, err := d.Run(context.Background(), p, 4)
			Expect(err).NotTo(HaveOccurred())

			Expect(s.Downloaded).To(Equal(2))
			Expect(s.FirstIndex).To(Equal(2))
			Expect(s.StoppedAt).To(Equal(4))
			Expect(hits.ns).To(Equal([]int{3, 2}))
		})
	})

	Describe("with Backward", func() {
		It("walks down from start to the first 404, then continues forward", func() {
			srv, hits := newServer(5, nil)
			p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
			Expect(err).NotTo(HaveOccurred())

			d := &downloader.Downloader{Workers: 1, OutDir: outDir, Backward: true, Out: &out}
			s, err := d.Run(context.Background(), p, 4)
			Expect(err).NotTo(HaveOccurred())

			Expect(s.Downloaded).To(Equal(5))
			Expect(s.FirstIndex).To(Equal(1))
			Expect(s.StoppedAt).To(Equal(6))
			expectFiles(1, 5)
			Expect(hits.ns).To(Equal([]int{3, 2, 1, 0, 4, 5, 6}))
		})

		It("stops the backward walk at a 403", func() {
			srv, hits := newServer(5, map[int]int{2: http.StatusForbidden})
			p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
			Expect(err).NotTo(HaveOccurred())

			d := &downloader.Downloader{Workers: 1, OutDir: outDir, Backward: true}
			s, err := d.Run(context.Background(), p, 4)
			Expect(err).NotTo(HaveOccurred())

			Expect(s.Downloaded).To(Equal(3))
			Expect(s.FirstIndex).To(Equal(3))
			Expect(s.StoppedAt).To(Equal(6))
			expectFiles(3, 5)
			Expect(hits.ns).To(Equal([]int{3, 2, 4, 5, 6}))
		})

		It("skips existing files while walking backward", func() {
			srv, hits := newServer(5, nil)
			p, err := pattern.Parse(srv.URL + "/f/{n}.txt")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(outDir, "2.txt"), []byte("local"), 0o644)).To(Succeed())

			d := &downloader.Downloader{Workers: 1, OutDir: outDir, Backward: true}
			s, err := d.Run(context.Background(), p, 4)
			Expect(err).NotTo(HaveOccurred())

			Expect(s.Downloaded).To(Equal(4))
			Expect(s.Skipped).To(Equal(1))
			Expect(s.FirstIndex).To(Equal(1))
			Expect(hits.ns).To(Equal([]int{3, 1, 0, 4, 5, 6}))
		})
	})
})
