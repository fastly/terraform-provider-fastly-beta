#!/usr/bin/env bash
# Public, INERT AWS documentation examples, taken from go-fastly recordings:
#   fastly/fixtures/s3s/create.yaml (go-fastly-main)
#   fastly/logging_s3_test.go
# These are not live AWS credentials and bucket-name is not a test-owned
# destination. The live test exercises Fastly configuration/state only,
# NOT S3 authentication or delivery.
TEST_S3_BUCKET='bucket-name'
TEST_S3_ACCESS_KEY='AKIAIOSFODNN7EXAMPLE'
TEST_S3_SECRET_KEY='wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY'
