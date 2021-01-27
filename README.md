# S3 consistency test

Test consistency of S3-compliant object stores.

## Object store eventual consistency

Object store are usually distributed storage and focus on availability/performances rather than consistency (see CAP
theorem).
Users may encounter unexpected behaviour with stale data being presented.

## Testing consistency

Tool runs the following consistency tests:

| Test | Description | 
| --- | --- |
| read-after-write         | Write a new file, try to read it immediately                                 |
| read-after-delete        | Delete a file and try to read it immediately                                 |
| read-after-overwrite     | Overwrite a file, read it immediately and check if data was updated          |
| list-after-create        | Write a new file, list the content of the bucket and check if file exists    |
| list-after-delete        | Delete a file, list the content of the bucket and check if file still exists |

Build the tool
```bash
./go-build-all.sh
```

Run the tool
```bash
./s3-consistency \
  --endpoint https://s3.gra.storage.cloud.ovh.net \ 
  --region gra \
  --threads 70 \
  --iterations 100 \
  --bucket s3-consistency
```

Available options
```bash
./build/darwin-amd64/s3-consistency --help
Usage of ./build/darwin-amd64/s3-consistency:
  -bucket string
        Bucket to use for test (default "s3-consistency")
  -chunk-size int
        Size in bytes of created files (default 1)
  -clean
        Clean bucket
  -endpoint string
        S3 endpoint to use (default "https://s3.us-east-1.amazonaws.com")
  -iterations int
        Number of iteration per thread per test. (default 5)
  -region string
        S3 endpoint to use (default "us-east-1")
  -threads int
        Number threads per test. (default 5)
```

## References
- https://aws.amazon.com/s3/consistency/
- https://lists.launchpad.net/openstack/msg06788.html
- https://julien.danjou.info/openstack-swift-consistency-analysis/
