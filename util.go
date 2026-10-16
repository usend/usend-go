package usend

import (
	"bytes"
	"encoding/json"
	"strconv"
)

func itoa(n int) string { return strconv.Itoa(n) }

// jsonList decodes either a bare JSON array or a { "data": [...] } envelope.
type jsonList[T any] struct{ items []T }

func (l *jsonList[T]) UnmarshalJSON(b []byte) error {
	if bytes.HasPrefix(bytes.TrimSpace(b), []byte("[")) {
		return json.Unmarshal(b, &l.items)
	}
	var env struct {
		Data []T `json:"data"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return err
	}
	l.items = env.Data
	return nil
}
