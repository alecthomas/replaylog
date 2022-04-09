# A type safe implementation of a replay log for Go

[![PkgGoDev](https://pkg.go.dev/badge/github.com/alecthomas/replaylog)](https://pkg.go.dev/github.com/alecthomas/replaylog) [![GHA Build](https://github.com/alecthomas/replaylog/actions/workflows/ci.yml/badge.svg)](https://github.com/alecthomas/replaylog/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/alecthomas/replaylog)](https://goreportcard.com/report/github.com/alecthomas/replaylog) [![Slack chat](https://img.shields.io/static/v1?logo=slack&style=flat&label=slack&color=green&message=gophers)](https://gophers.slack.com/messages/CN9DS8YF3)

A [replay log](https://ahmet.im/blog/the-replay-pattern/)
([related](https://ahmet.im/blog/the-replay-pattern/))
records the sequence of operations for mutating an empty
state to its final state. The previous final state can then be reconstructed
from the log by starting with an empty state, reading each operation
from the log, and applying it to the state until the final state is reached.


The Log is not safe for concurrent use across processes.

## Example

A simple key-value store:

```go
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

func main() {
    ops := []Op[KV]{&Set{}, &Delete{}}

    w, err := ioutil.TempFile("", "")

    log, err := New[KV](w, ops...)

    err = log.Append(&Set{Key: "foo", Value: "bar"})
    err = log.Append(&Set{Key: "bar", Value: "waz"})
    err = log.Append(&Delete{Key: "foo"})

    state := KV{}
	
    err = log.Rewind()
    err = log.Replay(state)

    fmt.Printf("%#v\n", state)
}
```
