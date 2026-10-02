package render

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-gl/gl/v4.3-core/gl"
)

// passTimer measures where a frame actually goes. It is a diagnostic, not part of
// the pipeline: when it is off every method is a no-op on a nil pointer, so the
// instrumented call sites stay unconditional.
//
// Each section is bracketed by glFinish. Without that the driver queues the work
// and returns immediately, so every pass measures as microseconds and the frame
// looks free.
type passTimer struct {
	acc   map[string]time.Duration
	order []string
	n     int
	cur   string
	t0    time.Time
}

func (t *passTimer) begin(name string) {
	if t == nil {
		return
	}
	gl.Finish()
	t.cur = name
	if _, ok := t.acc[name]; !ok {
		t.acc[name] = 0
		t.order = append(t.order, name)
	}
	t.t0 = time.Now()
}

func (t *passTimer) end() {
	if t == nil {
		return
	}
	gl.Finish()
	t.acc[t.cur] += time.Since(t.t0)
	t.cur = ""
}

// mark closes the section opened by the previous mark or begin, so a single pass
// can be split without nesting timers.
func (t *passTimer) mark(name string) {
	if t == nil {
		return
	}
	t.end()
	t.begin(name)
}

// report prints the running mean per pass once enough frames have accumulated,
// then resets so the next window is independent of the first.
func (t *passTimer) report(every int) {
	if t == nil {
		return
	}
	t.n++
	if t.n < every {
		return
	}
	type row struct {
		name string
		d    time.Duration
	}
	rows := make([]row, 0, len(t.order))
	var total time.Duration
	for _, n := range t.order {
		rows = append(rows, row{n, t.acc[n] / time.Duration(t.n)})
		total += t.acc[n] / time.Duration(t.n)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].d > rows[j].d })
	fmt.Printf("passes over %d frames, total mean %s:\n", t.n, total.Round(time.Microsecond*100))
	for _, r := range rows {
		fmt.Printf("  %-10s %7.2f ms\n", r.name, float64(r.d)/float64(time.Millisecond))
	}
	for k := range t.acc {
		t.acc[k] = 0
	}
	t.n = 0
}
