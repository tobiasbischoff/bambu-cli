package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type Format int

const (
	Human Format = iota
	Plain
	JSON
)

func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func WritePlainKV(w io.Writer, kv map[string]string) error {
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "%s=%s\n", k, kv[k]); err != nil {
			return err
		}
	}
	return nil
}
