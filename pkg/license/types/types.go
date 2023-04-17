package types

type LicenseField struct {
	Name        string                `json:"name,omitempty" yaml:"name,omitempty"`
	Title       string                `json:"title,omitempty" yaml:"title,omitempty"`
	Description string                `json:"description,omitempty" yaml:"description,omitempty"`
	Value       interface{}           `json:"value,omitempty" yaml:"value,omitempty"`
	ValueType   string                `json:"valueType,omitempty" yaml:"valueType,omitempty"`
	IsHidden    bool                  `json:"isHidden,omitempty" yaml:"isHidden,omitempty"`
	Signature   LicenseFieldSignature `json:"signature,omitempty" yaml:"signature,omitempty"`
}

type LicenseFieldSignature struct {
	V1 string `json:"v1,omitempty" yaml:"v1,omitempty"` // this is a base64 encoded string because yaml.Unmarshal doesn't automatically convert base64 to []byte like json.Unmarshal does
}

type LicenseFields map[string]LicenseField
