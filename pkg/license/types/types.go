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
	V1 string `json:"v1,omitempty" yaml:"v1,omitempty"` // v1beta1: base64 encoded MD5 signature
	V2 string `json:"v2,omitempty" yaml:"v2,omitempty"` // v1beta2: base64 encoded SHA-256 signature
}

type LicenseFields map[string]LicenseField
