package events

import "encoding/json"

func marshal(p Payload) ([]byte, error) {
	if p == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(p)
}
