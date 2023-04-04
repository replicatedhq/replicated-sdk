package types

type LicenseField struct {
	Name        string      `json:"name,omitempty"`
	Title       string      `json:"title,omitempty"`
	Description string      `json:"description,omitempty"`
	Value       interface{} `json:"value,omitempty"`
	ValueType   string      `json:"valueType,omitempty"`
	IsHidden    bool        `json:"isHidden,omitempty"`
	V1Signature []byte      `json:"v1Signature,omitempty"`
}

type LicenseFields map[string]LicenseField
