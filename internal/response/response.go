package response

import "encoding/json"

type OK map[string]any

type Error struct {
	OK      bool           `json:"ok"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func Marshal(v any) ([]byte, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func MarshalError(code, message string, details map[string]any) ([]byte, error) {
	return Marshal(Error{OK: false, Code: code, Message: message, Details: details})
}
