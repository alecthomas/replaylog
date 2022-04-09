package replaylog

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/alecthomas/assert/v2"
)

var ops = []Op[KV]{&Set{}, &Delete{}}

type KV map[string]string

type Set struct {
	Key   string `json:"k"`
	Value string `json:"v"`
}

func (s *Set) Apply(kv KV) error {
	kv[s.Key] = s.Value
	return nil
}

type Delete struct {
	Key string `json:"k"`
}

func (d *Delete) Apply(state KV) error {
	delete(state, d.Key)
	return nil
}

func TestLog(t *testing.T) {
	w, err := ioutil.TempFile(t.TempDir(), "") // nolint: varnamelen
	assert.NoError(t, err)

	t.Cleanup(func() {
		_ = w.Close()
		_ = os.Remove(w.Name())
	})

	t.Run("Append", func(t *testing.T) {
		t.Cleanup(func() { _ = w.Close() })

		log, err := New[KV](w, ops)
		assert.NoError(t, err)
		t.Cleanup(func() { _ = log.Close() })

		err = log.Append(&Set{Key: "foo", Value: "bar"})
		assert.NoError(t, err)
		err = log.Append(&Set{Key: "bar", Value: "waz"})
		assert.NoError(t, err)
		err = log.Append(&Delete{Key: "foo"})
		assert.NoError(t, err)
	})

	t.Run("ReplayThenAppend", func(t *testing.T) {
		state := KV{}

		r, err := os.OpenFile(w.Name(), os.O_RDWR, 0600)
		assert.NoError(t, err)
		defer r.Close()

		log, err := New[KV](r, ops)
		assert.NoError(t, err)
		err = log.Replay(state)
		assert.NoError(t, err)

		assert.Equal(t, KV{"bar": "waz"}, state)

		err = log.Append(&Set{Key: "foo", Value: "bar"})
		assert.NoError(t, err)
	})

	t.Run("ReplayAgain", func(t *testing.T) {
		state := KV{}

		r, err := os.OpenFile(w.Name(), os.O_RDWR, 0600)
		assert.NoError(t, err)
		defer r.Close()

		log, err := New[KV](r, ops)
		assert.NoError(t, err)
		err = log.Replay(state)
		assert.NoError(t, err)

		assert.Equal(t, KV{"bar": "waz", "foo": "bar"}, state)

		t.Run("Rewind", func(t *testing.T) {
			err = log.Rewind()
			assert.NoError(t, err)

			state := KV{}
			err = log.Replay(state)
			assert.NoError(t, err)

			assert.Equal(t, KV{"bar": "waz", "foo": "bar"}, state)
		})
	})
}
