## Depot CI

This repository now carries a Depot CI workflow set under `.depot/workflows/`.

Repo-side migration steps completed here:
- The repository's primary CI workflows were moved from `.github/workflows/` into `.depot/workflows/`.
- Workflow jobs were updated to use explicit Depot runner labels.
- The local reusable workflow reference was updated to point at `.depot/workflows/`.
- The only workflow intentionally left in `.github/workflows/` is `slsa.yml`, because the production publish path still dispatches that workflow through the GitHub Actions API.

Remaining Depot-side setup:
- Connect the GitHub organization to Depot CI with the Depot Code Access app.
- Import the GitHub secrets and variables used by these workflows into Depot CI.
- Merge these changes to register the Depot CI triggers.

Useful commands:

```bash
depot ci run --workflow .depot/workflows/validate.yml
depot ci run --workflow .depot/workflows/publish.yml
depot ci migrate secrets-and-vars
```

Current limitation:
- The production publish path in `dagger/publish.go` still dispatches `.github/workflows/slsa.yml` through the GitHub Actions API. That keeps provenance generation dependent on GitHub workflow dispatch until that call is moved to Depot CI's CLI or API.
