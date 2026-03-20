//filewriter.go

package filewriter

import (
	"time"
)

const (
	defaultBatchSize  = 600
	defaultFlushEvery = 2 * time.Second
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

type NDJSONOptions struct {
	BatchSize  int
	FlushEvery time.Duration
	LogHooks   LogHooks
}

func DefaultCSVOptions(mode Mode, atomic bool) CSVOptions {
	return CSVOptions{
		Mode:       mode,
		Atomic:     atomic,
		BatchSize:  defaultBatchSize,
		FlushEvery: defaultFlushEvery,
		LogHooks:   LogHooks{},
	}
}

func DefaultNDJSONOptions() NDJSONOptions {
	return NDJSONOptions{
		BatchSize:  defaultBatchSize,
		FlushEvery: defaultFlushEvery,
		LogHooks:   LogHooks{},
	}
}

// The two “future-proof” constructors used by external code.
// Their signatures never change.

func SafeNewCSVWriter(filename string, mode Mode) (*CSVWriter, error) {
	opts := DefaultCSVOptions(mode, false)
	return NewWriter(filename, opts)
}

func SafeNewAtomicCSVWriter(filename string) (*CSVWriter, error) {
	opts := DefaultCSVOptions(Overwrite, true)
	return NewWriter(filename, opts)
}

func SafeNewNDJSONWriter(filename string) (*NDJSONWriter, error) {
	opts := DefaultNDJSONOptions()
	return NewNDJSONWriter(filename, opts)
}
