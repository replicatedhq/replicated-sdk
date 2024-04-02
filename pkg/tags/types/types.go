package types

type InstanceTagData struct {
	IsForced bool              `json:"isForced"`
	Tags     map[string]string `json:"tags"`
}
