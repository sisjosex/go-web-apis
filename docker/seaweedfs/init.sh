#!/bin/sh
# Creates the dev and test buckets on the local S3 (INFRA-007 D3). Run once by the storage-init
# service; a bucket that already exists is left as it is.
until echo "s3.bucket.list" | weed shell -master=storage:9333 >/dev/null 2>&1; do sleep 1; done
for bucket in taypi24-media taypi24-documents taypi24-media-test taypi24-documents-test; do
  echo "s3.bucket.create -name $bucket" | weed shell -master=storage:9333 2>&1 | grep -v "already exists" || true
done
