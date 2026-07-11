module dagger/replicated-sdk

go 1.25.5

require github.com/1password/onepassword-sdk-go v0.1.5

require (
	github.com/dylibso/observe-sdk/go v0.0.0-20240819160327-2d926c5d788a // indirect
	github.com/extism/go-sdk v1.7.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/ianlancetaylor/demangle v0.0.0-20240805132620-81f5be970eca // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/tetratelabs/wabin v0.0.0-20230304001439-f6f874872834 // indirect
	github.com/tetratelabs/wazero v1.8.2 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc => go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.19.0

replace go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp => go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.19.0

replace go.opentelemetry.io/otel/log => go.opentelemetry.io/otel/log v0.19.0

replace go.opentelemetry.io/otel/sdk/log => go.opentelemetry.io/otel/sdk/log v0.19.0
