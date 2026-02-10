# Introduction

The Replicated SDK (software development kit) is a service that allows you to embed key Replicated features alongside your application.

[Replicated SDK Product Documentation](https://docs.replicated.com/vendor/replicated-sdk-overview)

## Go Client SDK

A Go client library is available at `pkg/replicatedclient` for interacting with the Replicated SDK API from your Go applications.

### Installation

```bash
go get github.com/replicatedhq/replicated-sdk/pkg/replicatedclient
```

### SDK Service Client

Use `New` to create a client that talks to the local Replicated SDK service:

```go
client := replicatedclient.New("http://replicated:3000")

license, err := client.GetLicenseInfo(ctx)
fields, err := client.GetLicenseFields(ctx)
```

### Direct Upstream Access

Use `NewUpstream` to talk directly to the Replicated API (`https://replicated.app`) without running the SDK service. This is useful for fetching license fields when the SDK service is not deployed.

```go
uc := replicatedclient.NewUpstream("your-license-id")

// Get all custom license fields
fields, err := uc.GetLicenseFields(ctx)

// Get a specific license field
field, err := uc.GetLicenseField(ctx, "seat_count")
```

#### Upstream Configuration Options

```go
// Custom endpoint (e.g. on-premise Replicated server)
uc := replicatedclient.NewUpstream("your-license-id",
	replicatedclient.WithEndpoint("https://custom.replicated.app"),
)

// Custom HTTP client
uc := replicatedclient.NewUpstream("your-license-id",
	replicatedclient.WithUpstreamHTTPClient(&http.Client{Timeout: 5 * time.Second}),
)
```

The upstream client authenticates using Basic Auth with the license ID as both username and password.