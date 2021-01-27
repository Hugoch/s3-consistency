# S3 consistency test

Test consistency of S3-compliant object stores.

## Object store eventual consistency

Object store are usually distributed storage and focus on availability/performances rather than consistency (see CAP
theorem).
Users may encounter unexpected behaviour with stale data being presented.

## Testing consistency

| Test | Description | 
| --- | --- |
| read-after-write         | Write a new file, try to read it immediately                                 |
| read-after-delete        | Delete a file and try to read it immediately                                 |
| read-after-overwrite     | Overwrite a file, read it immediately and check if data was updated          |
| list-after-create        | Write a new file, list the content of the bucket and check if file exists    |
| list-after-delete        | Delete a file, list the content of the bucket and check if file still exists |
