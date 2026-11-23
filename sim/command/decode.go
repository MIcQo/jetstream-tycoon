package command

import (
	"encoding/json"
	"fmt"
)

// ErrUnknownType is returned for a command type with no decoder.
var ErrUnknownType = &Error{Key: "errors.cmd.unknown_type"}

// ErrMalformed is returned when a command's JSON can't be decoded.
var ErrMalformed = &Error{Key: "errors.cmd.malformed"}

// decoders maps each stable wire type name to its decoder. Never rename a key.
var decoders = map[string]func([]byte) (Command, error){
	"setSpeed": decodeAs[SetSpeed],
}

func decodeAs[T Command](data []byte) (Command, error) {
	var c T
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err) //nolint:errorlint // keep ErrMalformed as the wrapped error
	}
	return c, nil
}

// Decode builds a Command from its JSON form `{"type": "...", ...fields}`.
func Decode(data []byte) (Command, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err) //nolint:errorlint // keep ErrMalformed as the wrapped error
	}
	decode, ok := decoders[head.Type]
	if !ok {
		return nil, ErrUnknownType
	}
	return decode(data)
}
