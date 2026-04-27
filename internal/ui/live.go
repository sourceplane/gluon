package ui

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// LiveRegion renders a transient block of "in flight" rows below the cursor
// while still allowing persistent lines to be printed above. On non-TTY
// writers it degrades to plain append-only output: SetRow/RemoveRow become
// no-ops and Print just writes a line.
type LiveRegion struct {
	w            io.Writer
	tty          bool
	color        bool
	mu           sync.Mutex
	rows         []liveRow
	rowIndex     map[string]int
	frame        int
	stop         chan struct{}
	done         chan struct{}
	lastRendered int
}

type liveRow struct {
	key   string
	label string
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewLiveRegion constructs a live region.
func NewLiveRegion(w io.Writer, tty, color bool) *LiveRegion {
	return &LiveRegion{
		w:        w,
		tty:      tty,
		color:    color,
		rowIndex: map[string]int{},
	}
}

// Start begins the spinner render goroutine. No-op for non-TTY.
func (l *LiveRegion) Start() {
	if !l.tty {
		return
	}
	l.stop = make(chan struct{})
	l.done = make(chan struct{})
	go l.loop()
}

// Stop ends the render goroutine and clears the live region.
func (l *LiveRegion) Stop() {
	if !l.tty || l.stop == nil {
		return
	}
	close(l.stop)
	<-l.done
	l.stop = nil
	l.done = nil
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clearRegion()
	l.rows = nil
	l.rowIndex = map[string]int{}
}

func (l *LiveRegion) loop() {
	t := time.NewTicker(90 * time.Millisecond)
	defer t.Stop()
	defer close(l.done)
	for {
		select {
		case <-l.stop:
			return
		case <-t.C:
			l.mu.Lock()
			l.frame++
			l.draw()
			l.mu.Unlock()
		}
	}
}

// SetRow inserts or updates a row by key. label should be a short single line.
func (l *LiveRegion) SetRow(key, label string) {
	if !l.tty {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if idx, ok := l.rowIndex[key]; ok {
		l.rows[idx].label = label
	} else {
		l.rowIndex[key] = len(l.rows)
		l.rows = append(l.rows, liveRow{key: key, label: label})
	}
	l.draw()
}

// RemoveRow drops a row from the live region.
func (l *LiveRegion) RemoveRow(key string) {
	if !l.tty {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	idx, ok := l.rowIndex[key]
	if !ok {
		return
	}
	l.rows = append(l.rows[:idx], l.rows[idx+1:]...)
	delete(l.rowIndex, key)
	for k, v := range l.rowIndex {
		if v > idx {
			l.rowIndex[k] = v - 1
		}
	}
	l.draw()
}

// Print writes a persistent line above the live region.
func (l *LiveRegion) Print(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clearRegion()
	fmt.Fprintln(l.w, line)
	l.draw()
}

// PrintBlock writes multiple persistent lines above the live region.
func (l *LiveRegion) PrintBlock(lines []string) {
	if len(lines) == 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clearRegion()
	for _, ln := range lines {
		fmt.Fprintln(l.w, ln)
	}
	l.draw()
}

// clearRegion clears any previously rendered live rows. Caller holds mu.
func (l *LiveRegion) clearRegion() {
	if !l.tty || l.lastRendered <= 0 {
		return
	}
	fmt.Fprintf(l.w, "\x1b[%dA\x1b[J", l.lastRendered)
	l.lastRendered = 0
}

// draw renders the current rows. Caller holds mu.
func (l *LiveRegion) draw() {
	if !l.tty {
		return
	}
	l.clearRegion()
	if len(l.rows) == 0 {
		return
	}
	frame := spinnerFrames[l.frame%len(spinnerFrames)]
	for _, row := range l.rows {
		fmt.Fprintf(l.w, "    %s %s\n", Cyan(l.color, frame), Dim(l.color, row.label))
	}
	l.lastRendered = len(l.rows)
}
