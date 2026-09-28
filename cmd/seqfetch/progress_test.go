package main

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("isTerminal", func() {
	It("is false for an in-memory writer", func() {
		Expect(isTerminal(&bytes.Buffer{})).To(BeFalse())
	})

	It("is false for a regular file", func() {
		f, err := os.Create(filepath.Join(GinkgoT().TempDir(), "out"))
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()
		Expect(isTerminal(f)).To(BeFalse())
	})
})

var _ = Describe("termProgress", func() {
	var (
		out bytes.Buffer
		tp  *termProgress
	)

	BeforeEach(func() {
		out.Reset()
		tp = newTermProgress(&out)
	})

	// wait asserts that tp.Wait returns, i.e. no bar was left running.
	wait := func() {
		GinkgoHelper()
		done := make(chan struct{})
		go func() {
			tp.Wait()
			close(done)
		}()
		Eventually(done).Should(BeClosed())
	}

	It("completes the bar once every byte of a known size arrived", func() {
		w := tp.Track("a.pdf", 10).(*barWriter)
		n, err := w.Write(make([]byte, 4))
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(4))
		Expect(w.bar.Current()).To(Equal(int64(4)))
		Expect(w.bar.Completed()).To(BeFalse())

		_, err = w.Write(make([]byte, 6))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Close()).To(Succeed())
		Expect(w.bar.Completed()).To(BeTrue())
		Expect(w.bar.Aborted()).To(BeFalse())
		wait()
	})

	It("aborts the bar of a transfer that stopped short", func() {
		w := tp.Track("a.pdf", 10).(*barWriter)
		_, err := w.Write(make([]byte, 3))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Close()).To(Succeed())
		Expect(w.bar.Aborted()).To(BeTrue())
		wait()
	})

	It("tracks an unknown size as a spinner that ends on Close", func() {
		w := tp.Track("a.pdf", -1).(*barWriter)
		_, err := w.Write(make([]byte, 3))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.bar.Current()).To(Equal(int64(3)))
		Expect(w.Close()).To(Succeed())
		Expect(w.bar.AbortedOrCompleted()).To(BeTrue())
		wait()
	})
})
