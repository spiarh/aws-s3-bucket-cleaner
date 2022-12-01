# AWS S3 Bucket Cleaner

This small tool helps with cleaning object from an AWS s3 bucket without when
versioning is not enabled. Objects older than 90 days are cleaned-up by default.

The bucket credentials must be provided as environment variables:
```
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
```

The bucket configuration can be provided as environment variables or on the CLI:

```
BUCKET_NAME
BUCKET_REGION
```

## Integration with AWS S3 Operator

It is useful in combination with a bucket created from an `ObjectBucketClaim`
on Managed Platform Plus.

1. Assuming the following `ObjectBucketClaim` is created in a namespace, the
bucket configuration is stored into the `ConfigMap` named `static-files` and
the bucket credentials are stored into the `Secret` named `static-files`.

```yaml
apiVersion: objectbucket.io/v1alpha1
kind: ObjectBucketClaim
metadata:
  annotations:
    objectbucket.io/reclaimPolicy: Delete
  labels:
    app: static-files
  name: static-files
spec:
  generateBucketName: cluster-backup
  storageClassName: aws-s3
```

2. This tool can then be executed in a `CronJob` that only needs to use the
environment variables from the `ConfigMap` and `Secret`.

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  labels:
    app: static-files
  name: bucket-cleaner-static-files
spec:
  concurrencyPolicy: Forbid
  failedJobsHistoryLimit: 7
  jobTemplate:
    metadata:
      labels:
        app: static-files
    spec:
      template:
        metadata:
          labels:
            app: static-files
        spec:
          activeDeadlineSeconds: 300
          containers:
          - envFrom:
            - secretRef:
                name: static-files
            - configMapRef:
                name: static-files
            image: aws-s3-bucket-cleaner:0.1
            imagePullPolicy: IfNotPresent
            name: bucket-cleaner-static-files
          restartPolicy: OnFailure
          serviceAccountName: default
  schedule: 30 3 * * *
  successfulJobsHistoryLimit: 7
```

## Usage

```
Usage of aws-s3-bucket-cleaner:
  -bucket string
    	Bucket name, default to BUCKET_NAME environment variable
  -dry-run
    	Do not actually delete any object
  -max-age duration
    	Delete objects older than the max age, 90d by default (default 2160h0m0s)
  -region string
    	Bucket region, default to BUCKET_REGION environment variable
```
