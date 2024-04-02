package types

import (
	"encoding/base64"
	"encoding/json"
)

type InstanceTagData struct {
	IsForced bool              `json:"isForced"`
	Tags     map[string]string `json:"tags"`
}

func (i InstanceTagData) IsEmpty() bool {
	return len(i.Tags) == 0
}

func (i InstanceTagData) ToBase64() (string, error) {
	b, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
