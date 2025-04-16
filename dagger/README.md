 dagger call validate --progress=plain --op-service-account=env:OP_SERVICE_ACCOUNT
 
# Testing the release process

You can test locally by pushing to ttl.sh.
This doesn't test SLSA generation though.

```
dagger call publish --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT \
    --version 0.0.1
```

If you have creds, you can test a staging release.  The OP_SERVICE_ACCOUNT_PRODUCTION has access to Developer Automation Production:

```
dagger call publish --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT \
    --op-service-account-production=env:OP_SERVICE_ACCOUNT_PRODUCTION \
    --dev=false \
    --staging=true \
    --version 1.6.0-beta.5
```

