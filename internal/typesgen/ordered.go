package typesgen

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// OrderedObject preserves JSON object key order for deterministic TypeScript output.
type OrderedObject struct {
	Keys   []string
	Values map[string]json.RawMessage
}

// ParseOrderedObject decodes a JSON object while retaining key insertion order.
func ParseOrderedObject(raw json.RawMessage) (OrderedObject, error) {
	result := OrderedObject{
		Keys:   make([]string, 0),
		Values: make(map[string]json.RawMessage),
	}
	if len(raw) == 0 || string(raw) == "null" {
		return result, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return result, err
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return result, fmt.Errorf("expected JSON object")
	}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return result, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return result, fmt.Errorf("expected object key string")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return result, err
		}
		if _, exists := result.Values[key]; !exists {
			result.Keys = append(result.Keys, key)
		}
		result.Values[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return result, err
	}
	return result, nil
}

// Get returns the raw value for a key.
func (object OrderedObject) Get(key string) (json.RawMessage, bool) {
	value, ok := object.Values[key]
	return value, ok
}

// Empty reports whether the object has no keys.
func (object OrderedObject) Empty() bool {
	return len(object.Keys) == 0
}
