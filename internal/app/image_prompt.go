package app

import (
	"encoding/json"
	"fmt"
)

// promptImagePayload uses the regular, compact conversation request for a
// drawing prompt. The media-tool payload is a separate Gemini web flow.
func promptImagePayload(prompt string) ([]byte, error) {
	inner, err := json.Marshal([]interface{}{[]interface{}{prompt}, nil, nil, "gemini-3-pro"})
	if err != nil {
		return nil, err
	}
	return json.Marshal([]interface{}{nil, string(inner)})
}

func promptImageModelHeader(hexID string) string {
	return fmt.Sprintf(`[1,null,null,null,%q,null,null,0,[4],null,null,1]`, hexID)
}
