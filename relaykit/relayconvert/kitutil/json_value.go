package kitutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// DecodeJSONValue decodes one JSON value into a generic tree of:
// nil, bool, string, json.Number, []any and map[string]any.
//
// Unlike the codec Unmarshal path it preserves enough information for strict
// configuration validation: numbers keep their literal json.Number form and a
// duplicate object key at any depth is reported instead of silently collapsing
// to the last value. Trailing tokens after the first value are rejected.
func DecodeJSONValue(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := decodeJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("trailing json after value")
		}
		return nil, err
	}
	return value, nil
}

func decodeJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("object key is not a string")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("duplicate object key %q", key)
			}
			value, err := decodeJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := decodeJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected json delimiter %q", delim)
	}
}

// TopLevelFieldOccurrences scans only the top-level object keys of data and
// reports how many times the exact canonical key and case-insensitive variants
// appear. It skips values without validating them, so duplicates of unrelated
// legacy keys do not fail this scan. A non-object top level is an error.
func TopLevelFieldOccurrences(data []byte, canonical string) (exact, folded int, err error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, 0, err
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return 0, 0, errors.New("configuration is not a json object")
	}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return 0, 0, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return 0, 0, errors.New("object key is not a string")
		}
		if err := skipJSONValue(decoder); err != nil {
			return 0, 0, err
		}
		if key == canonical {
			exact++
		}
		if strings.EqualFold(key, canonical) {
			folded++
		}
	}
	if _, err := decoder.Token(); err != nil {
		return 0, 0, err
	}
	return exact, folded, nil
}

// skipJSONValue consumes one JSON value without building it or checking for
// duplicate keys.
func skipJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		for decoder.More() {
			if _, err := decoder.Token(); err != nil {
				return err
			}
			if err := skipJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := skipJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected json delimiter %q", delim)
	}
	_, err = decoder.Token()
	return err
}
