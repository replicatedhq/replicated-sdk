package types

import (
	"encoding/base64"
	"encoding/json"

	"github.com/pkg/errors"
)

type InstanceTagData struct {
	Force bool              `json:"force"`
	Tags  map[string]string `json:"tags"`
}

func (i InstanceTagData) IsEmpty() bool {
	return len(i.Tags) == 0
}

func (i InstanceTagData) MarshalBase64() ([]byte, error) {
	b, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	return []byte(base64.StdEncoding.EncodeToString(b)), nil
}

func (i *InstanceTagData) UnmarshalBase64(bs []byte) error {
	b, err := base64.StdEncoding.DecodeString(string(bs))
	if err != nil {
		return errors.Wrap(err, "failed to decode instance-tag data base64")
	}

	if err := json.Unmarshal(b, &i); err != nil {
		return errors.Wrap(err, "failed to unmarshal json")
	}
	return nil
}
