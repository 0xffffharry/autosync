package util

import (
	"encoding/json"
	"fmt"
)

type Listable[T any] []T

func (l *Listable[T]) UnmarshalJSON(content []byte) error {
	var list []T
	err1 := json.Unmarshal(content, &list)
	if err1 == nil {
		*l = list
		return nil
	}
	var single T
	err2 := json.Unmarshal(content, &single)
	if err2 == nil {
		*l = []T{single}
		return nil
	}
	return fmt.Errorf("%w | %w", err1, err2)
}

func (l *Listable[T]) MarshalJSON() ([]byte, error) {
	switch len(*l) {
	case 0:
		return nil, nil
	case 1:
		return json.Marshal((*l)[0])
	default:
		return json.Marshal([]T(*l))
	}
}
