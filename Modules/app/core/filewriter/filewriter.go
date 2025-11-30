//filewriter.go

package filewriter

import (
	"time"
)

// ----------
// PUBLIC API
// ----------

type Mode int

const (
	Overwrite Mode = iota
	Append
)

// Optional log callback hooks
type LogHooks struct {
	OnBatchFlush func(size int, duration time.Duration)
	OnError      func(err error)
}

// Stable configuration used by NewWriter
type CSVOptions struct {
	Mode       Mode
	Atomic     bool
	BatchSize  int
	FlushEvery time.Duration
	LogHooks   LogHooks
}

// The two “future-proof” constructors used by external code.
// Their signatures never change.

func SafeNewCSVWriter(filename string, mode Mode) (*CSVWriter, error) {
	opts := CSVOptions{
		Mode:       mode,
		Atomic:     false,
		BatchSize:  600,
		FlushEvery: 2 * time.Second,
		LogHooks:   LogHooks{},
	}
	return NewWriter(filename, opts)
}

func SafeNewAtomicCSVWriter(filename string) (*CSVWriter, error) {
	opts := CSVOptions{
		Mode:       Overwrite,
		Atomic:     true,
		BatchSize:  600,
		FlushEvery: 2 * time.Second,
		LogHooks:   LogHooks{},
	}
	return NewWriter(filename, opts)
}
