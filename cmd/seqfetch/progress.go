package main

import (
	"io"
	"os"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// isTerminal reports whether w is an interactive terminal, where live
// progress bars make sense.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// barWriter feeds the bytes of one transfer into its bar.
type barWriter struct {
	bar *mpb.Bar
}

func (b *barWriter) Write(p []byte) (int, error) {
	b.bar.IncrBy(len(p))
	return len(p), nil
}

// Close removes the bar. A bar that did not reach its total was aborted
// (an error mid-transfer or an unknown size), so it is dropped explicitly.
func (b *barWriter) Close() error {
	if !b.bar.Completed() {
		b.bar.Abort(true)
	}
	return nil
}

// termProgress draws one bar per transfer in flight. Log lines written to
// it are printed above the bars, so it doubles as the downloader's Out.
type termProgress struct {
	*mpb.Progress
}

func newTermProgress(out io.Writer) *termProgress {
	return &termProgress{mpb.New(mpb.WithOutput(out), mpb.WithWidth(60))}
}

// Track implements downloader.Progress. A known size gets a bar with
// percentage, counters and speed; an unknown one gets a spinner with the
// bytes received so far.
func (t *termProgress) Track(name string, total int64) io.WriteCloser {
	prepend := mpb.PrependDecorators(decor.Name(name, decor.WCSyncSpaceR))
	if total <= 0 {
		bar := t.New(0, mpb.SpinnerStyle().PositionLeft(), prepend, mpb.BarRemoveOnComplete(),
			mpb.AppendDecorators(decor.CurrentKibiByte("% .1f")))
		return &barWriter{bar: bar}
	}
	bar := t.AddBar(total, prepend, mpb.BarRemoveOnComplete(),
		mpb.AppendDecorators(
			decor.Percentage(decor.WC{W: 5}),
			decor.CountersKibiByte(" % .1f / % .1f"),
			decor.AverageSpeed(decor.SizeB1024(0), " % .1f"),
		))
	return &barWriter{bar: bar}
}
