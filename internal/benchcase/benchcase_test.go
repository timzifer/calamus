package benchcase

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestFailuresGiveNoResult(t *testing.T) {
	good := Encoder{Name: "good", Encode: func(w io.Writer) error {
		_, err := w.Write([]byte("ok"))
		return err
	}}
	bad := Encoder{Name: "bad", Encode: func(io.Writer) error { return errors.New("boom") }}
	invalid := Encoder{Name: "invalid", Encode: func(w io.Writer) error {
		_, err := w.Write([]byte("no"))
		return err
	}}
	check := func(want, got []byte) error {
		if !bytes.Equal(want, got) {
			return errors.New("differs")
		}
		return nil
	}
	for _, c := range []struct {
		name   string
		ref    Encoder
		encs   []Encoder
		failed bool
	}{
		{"valid", good, []Encoder{good}, false},
		{"failing reference", bad, []Encoder{good}, true},
		{"failing encoder", good, []Encoder{bad}, true},
		{"invalid output", good, []Encoder{invalid}, true},
	} {
		var failed bool
		testing.Benchmark(func(b *testing.B) {
			// b.Fatal ends the function; the deferred call still sees it.
			defer func() { failed = b.Failed() }()
			Run(b, c.ref, c.encs, check)
		})
		if failed != c.failed {
			t.Errorf("%s: failed %v, want %v", c.name, failed, c.failed)
		}
	}
}
