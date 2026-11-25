#!/usr/bin/env bash
# The public go-fastly S3 recording uses a placeholder bucket.
# The runner assembles its public example keys at runtime to prevent
# secret-scanner false positives in the checked-in fixtures.
# These values are not connected to an actual AWS destination.
TEST_S3_BUCKET='bucket-name'
