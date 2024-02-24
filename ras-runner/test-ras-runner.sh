#!/bin/bash

set -euo pipefail

source .env

export AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY
export AWS_REGION
export AWS_BUCKET

docker build -t ras-runner .

docker run --rm \
    -e AWS_ACCESS_KEY_ID \
    -e AWS_SECRET_ACCESS_KEY \
    -e AWS_REGION \
    -e AWS_BUCKET \
    ras-runner \
    ./main '{"s3key":"prototype/huc-020801050103/sims/jobs/huc-020801050103-mb-nd-125.json"}'