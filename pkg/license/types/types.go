package types

type LicenseField struct {
	Name        string                `json:"name,omitempty"`
	Title       string                `json:"title,omitempty"`
	Description string                `json:"description,omitempty"`
	Value       interface{}           `json:"value,omitempty"`
	ValueType   string                `json:"valueType,omitempty"`
	IsHidden    bool                  `json:"isHidden,omitempty"`
	Signature   LicenseFieldSignature `json:"signature,omitempty"`
}

type LicenseFieldSignature struct {
	V1 []byte `json:"v1,omitempty"`
}

type LicenseFields map[string]LicenseField
