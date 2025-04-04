# Introduction

The Replicated SDK (software development kit) is a service that allows you to embed key Replicated features alongside your application. 

[Replicated SDK Product Documentation](https://docs.replicated.com/vendor/replicated-sdk-overview) 

## Using an Existing Secret

By default, the Helm chart creates a new secret for storing configuration data. You can specify an existing secret instead of creating a new one by setting the `existingSecret.name` value:

```yaml
existingSecret:
  # Name of the existing secret to use
  name: "my-existing-secret"
```

This is useful when you want to manage the secret separately from the Helm chart deployment, such as when using external secret management solutions.

### Example

```bash
helm install replicated-sdk replicated/replicated-sdk \
  --set existingSecret.name=my-existing-secret
```

Note: The existing secret must contain the required configuration data with the same keys as the default secret. If the secret doesn't exist or doesn't have the expected keys, the deployment will fail.
