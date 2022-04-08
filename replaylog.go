// Package replaylog provides a type safe implementation of a replay log.
//
// A replay log records the sequence of operations for mutating an empty
// state to its final state. The previous final state can then be reconstructed
// from the log by starting with an empty state, reading each operation
// from the log, and applying it to the state until the final state is reached.
//
// The Log is NOT safe for concurrent use between multiple processes. It is safe
// for concurrent use within a single Go process.
package replaylog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
)

// Op to apply to mutate the State.
type Op[State any] interface {
	Apply(state State) error
}

// Log for recording mutation operations on State.
type Log[State any] struct {
	lock   sync.Mutex
	f      File
	enc    *json.Encoder
	events map[reflect.Type]int
	ops    []Op[State]
}

type entry struct {
	Kind  int             `json:"k"`
	Event json.RawMessage `json:"e"`
}

// The File interface required by the Log.
type File interface {
	// Sync commits the current contents of the file to stable storage.
	Sync() error
	io.Reader
	io.Writer
	io.Closer
	io.Seeker
}

// New creates a new Log for recording mutation operations against the type State.
//
// "ops" is the ordered set of mutation types supported on State with the
// following constraints: they must be in the same order between instantiations,
// individual ops must not be removed, new op's must be appended, each op must be
// JSON-encodable, and must be forwards and backwards compatible.
func New[State any](f File, ops ...Op[State]) (*Log[State], error) { // nolint: varnamelen
	eventTypes := make(map[reflect.Type]int, len(ops))
	for i, op := range ops {
		eventTypes[reflect.TypeOf(op)] = i
	}
	return &Log[State]{
		f:      f,
		ops:    ops,
		enc:    json.NewEncoder(f),
		events: eventTypes,
	}, nil
}

// Append an Op to the log.
func (l *Log[State]) Append(event Op[State]) error {
	l.lock.Lock()
	defer l.lock.Unlock()
	kind, ok := l.events[reflect.TypeOf(event)]
	if !ok {
		return fmt.Errorf("unregistered event of type %T", event)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("could not encode event of type %T: %w", event, err)
	}
	e := entry{Kind: kind, Event: data}
	err = l.enc.Encode(e)
	if err != nil {
		return fmt.Errorf("failed to encode event: %w", err)
	}
	err = l.f.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync log: %w", err)
	}
	return nil
}

// Replay operations previously recorded into the log into "dest".
//
// After Replay, Append can be used to continue
func (l *Log[State]) Replay(dest State) error {
	dec := json.NewDecoder(l.f)
	dec.DisallowUnknownFields()
	for {
		logEntry := entry{}
		err := dec.Decode(&logEntry)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("corrupt log entry: %w", err)
		}
		opType := reflect.TypeOf(l.ops[logEntry.Kind])
		var event Op[State]
		if opType.Kind() == reflect.Ptr {
			event = reflect.New(opType.Elem()).Interface().(Op[State])
		} else {
			event = reflect.New(opType).Elem().Interface().(Op[State])
		}
		err = json.Unmarshal(logEntry.Event, event)
		if err != nil {
			return fmt.Errorf("could not decode event of kind %d into type %s: %w", logEntry.Kind, opType, err)
		}
		err = event.Apply(dest)
		if err != nil {
			return fmt.Errorf("could not apply event: %w", err)
		}
	}
	return nil
}

// Rewind to beginning of log.
func (l *Log[State]) Rewind() error {
	_, err := l.f.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to rewind log: %w", err)
	}
	return nil
}

// Close the Log file.
func (l *Log[State]) Close() error {
	return l.f.Close()
}
