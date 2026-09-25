// Package downloader fetches a sequence of URLs, stopping when an index
// returns HTTP 404 or 403. Optionally it first walks backwards from the
// start index to find the beginning of the sequence.
package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/oswaldom-code/seqfetch/internal/pattern"
)

// Downloader configures a run. Zero values fall back to sane defaults.
type Downloader struct {
	Client   *http.Client // defaults to http.DefaultClient
	Workers  int          // defaults to 4
	OutDir   string       // destination directory, must exist
	Out      io.Writer    // progress output, defaults to io.Discard
	Backward bool         // walk from start-1 down to the first 404/403 before going forward
	Limit    int          // process at most this many indices in total; 0 means unlimited
}

// Summary reports what a run did.
type Summary struct {
	Downloaded int     // files written successfully
	Skipped    int     // files already present locally, not requested
	FirstIndex int     // lowest index that exists (== start unless Backward found more)
	StoppedAt  int     // first index above start not processed: a 404/403 or the Limit cut-off
	Failures   []error // other errors; the run continues past them
}

type outcome struct {
	n        int
	notFound bool
	skipped  bool
	err      error
}

func (d *Downloader) client() *http.Client {
	if d.Client == nil {
		return http.DefaultClient
	}
	return d.Client
}

func (d *Downloader) workers() int {
	if d.Workers <= 0 {
		return 4
	}
	return d.Workers
}

func (d *Downloader) out() io.Writer {
	if d.Out == nil {
		return io.Discard
	}
	return d.Out
}

// lowerBound atomically sets bound to min(bound, n).
func lowerBound(bound *atomic.Int64, n int) {
	for {
		cur := bound.Load()
		if int64(n) >= cur || bound.CompareAndSwap(cur, int64(n)) {
			return
		}
	}
}

// fetchOne downloads url into dest. It reports notFound for HTTP 404 and
// 403 and removes any partially written file on error.
func (d *Downloader) fetchOne(ctx context.Context, url, dest string) (notFound bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	resp, err := d.client().Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return true, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return false, fmt.Errorf("unexpected status %s", resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return false, err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(dest)
		return false, err
	}
	if err := f.Close(); err != nil {
		os.Remove(dest)
		return false, err
	}
	return false, nil
}

// process handles a single index: skips it if the file already exists
// locally, otherwise downloads it.
func (d *Downloader) process(ctx context.Context, p *pattern.Pattern, n int) outcome {
	url := p.URL(n)
	dest := filepath.Join(d.OutDir, p.FileName(n))
	if _, err := os.Stat(dest); err == nil {
		return outcome{n: n, skipped: true}
	}
	notFound, err := d.fetchOne(ctx, url, dest)
	if err != nil {
		err = fmt.Errorf("%s: %w", url, err)
	}
	return outcome{n: n, notFound: notFound, err: err}
}

// report folds an outcome into the summary and prints a progress line.
func (d *Downloader) report(p *pattern.Pattern, s *Summary, r outcome) {
	switch {
	case r.err != nil:
		s.Failures = append(s.Failures, r.err)
		fmt.Fprintf(d.out(), "FAIL  %s\n", r.err)
	case r.notFound:
		fmt.Fprintf(d.out(), "MISS  %s\n", p.URL(r.n))
	case r.skipped:
		s.Skipped++
		fmt.Fprintf(d.out(), "SKIP  %s (already exists)\n", p.FileName(r.n))
	default:
		s.Downloaded++
		fmt.Fprintf(d.out(), "OK    %s\n", p.FileName(r.n))
	}
}

// walkBackward processes start-1, start-2, ... sequentially until an index
// is missing, budget indices have been processed, ctx is cancelled or 0 is
// passed. It returns the lowest index found to exist and how many indices
// it processed.
func (d *Downloader) walkBackward(ctx context.Context, p *pattern.Pattern, start, budget int, s *Summary) (first, used int) {
	first = start
	for n := start - 1; n >= 0 && used < budget && ctx.Err() == nil; n-- {
		r := d.process(ctx, p, n)
		d.report(p, s, r)
		used++
		if r.notFound {
			break
		}
		if r.err == nil {
			first = n
		}
	}
	return first, used
}

func (d *Downloader) worker(ctx context.Context, p *pattern.Pattern, bound *atomic.Int64, jobs <-chan int, results chan<- outcome) {
	for n := range jobs {
		if int64(n) > bound.Load() {
			continue // a lower index is already known to be missing
		}
		r := d.process(ctx, p, n)
		if r.notFound {
			lowerBound(bound, n)
		}
		results <- r
	}
}

// produce feeds indices from start upward until one exceeds bound, budget
// indices have been issued or ctx is cancelled, counting them in *issued.
// Every write to *issued happens before jobs is closed, so the caller may
// read it once the workers have exited. Indices already in flight when a
// miss is detected still finish; runForward discards what they wrote.
func produce(ctx context.Context, start, budget int, bound *atomic.Int64, jobs chan<- int, issued *int) {
	defer close(jobs)
	for n := start; *issued < budget && int64(n) <= bound.Load(); n++ {
		select {
		case <-ctx.Done():
			return
		case jobs <- n:
			*issued++
		}
	}
}

// discard removes the file written for an index past the first missing
// one (it was already in flight when the miss was detected) so the output
// directory holds only the contiguous run, and undoes its count.
func (d *Downloader) discard(p *pattern.Pattern, s *Summary, n int) {
	name := p.FileName(n)
	if err := os.Remove(filepath.Join(d.OutDir, name)); err != nil {
		s.Failures = append(s.Failures, err)
		fmt.Fprintf(d.out(), "FAIL  %s\n", err)
		return
	}
	s.Downloaded--
	fmt.Fprintf(d.out(), "DROP  %s (past the first missing index)\n", name)
}

// runForward downloads start, start+1, ... concurrently until the first
// missing index or until budget indices have been issued. Files downloaded
// for indices past that miss are discarded. It reports whether a missing
// index was reached.
func (d *Downloader) runForward(ctx context.Context, p *pattern.Pattern, start, budget int, s *Summary) (reachedMiss bool) {
	var bound atomic.Int64
	bound.Store(math.MaxInt64)

	jobs := make(chan int)
	results := make(chan outcome)

	var wg sync.WaitGroup
	for i := 0; i < d.workers(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.worker(ctx, p, &bound, jobs, results)
		}()
	}
	var issued int
	go produce(ctx, start, budget, &bound, jobs, &issued)
	go func() {
		wg.Wait() // workers exit only after jobs is closed, so issued is settled
		close(results)
	}()

	var downloaded []int
	for r := range results {
		d.report(p, s, r)
		if r.err == nil && !r.notFound && !r.skipped {
			downloaded = append(downloaded, r.n)
		}
	}
	for _, n := range downloaded {
		if int64(n) > bound.Load() {
			d.discard(p, s, n)
		}
	}
	s.StoppedAt = min(int(bound.Load()), start+issued)
	return bound.Load() != math.MaxInt64
}

// Run downloads p from index start upward until the first 404/403. With
// Backward set it first walks down from start-1 to the first missing index.
// With Limit set it stops after that many indices regardless of misses.
// Files that already exist in OutDir are skipped without a request, so an
// interrupted run can be resumed by re-running the same command.
func (d *Downloader) Run(ctx context.Context, p *pattern.Pattern, start int) (Summary, error) {
	budget := d.Limit
	if budget <= 0 {
		budget = math.MaxInt
	}

	s := Summary{FirstIndex: start, StoppedAt: start}
	if d.Backward {
		var used int
		s.FirstIndex, used = d.walkBackward(ctx, p, start, budget, &s)
		budget -= used
	}
	reachedMiss := false
	if budget > 0 {
		reachedMiss = d.runForward(ctx, p, start, budget, &s)
	}

	if err := ctx.Err(); err != nil {
		return s, err
	}
	if d.Limit <= 0 && !reachedMiss {
		return s, errors.New("run ended without reaching a missing index")
	}
	return s, nil
}
