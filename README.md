# Bill of Materials Management (bom-mgmt)

Go automation for uploading bits from a Bill of Materials to a Minio server.

## Bill of Materials

This product utilizes a YAML file structure that lays out a bill of materials like this:
```
bits:
- name: abc.txt
  contentType: text/plain
- name: xyz.zip
  contentType: application/zip
```

## Build from the source

`bom-mgmt` is written in [Go](https://golang.org/).
To build the binary yourself, follow these steps:

* Install `Go`.
* Install [Glide](https://github.com/Masterminds/glide), a dependency management tool for Go.
* Clone the repo:
  - `mkdir -p $(go env GOPATH)/src/github.com/pivotalservices`
  - `cd $(go env GOPATH)/src/github.com/pivotalservices`
  - `git clone git@github.com:pivotalservices/bom-mgmt.git`
* Install dependencies:
  - `cd bom-mgmt`
  - `glide install`
  - `go build -o bom-mgmt cmd/bom-mgmt/main.go`

To cross compile, set the `$GOOS` and `$GOARCH` environment variables.
For example: `GOOS=linux GOARCH=amd64 go build`.

## Uploading
```
export MINIO_HOST="localhost:9000"
export MINIO_ACCESS_KEY="key"
export MINIO_SECRET="secretsquirrel"

bom-mgmt upload-bits --bits "/Users/foo/test-bits" --bom "bom.yml" --bucket "bar"
```

## Parameters

All of the necessary parameters can either be passed on the command line or as environment variables.

| CLI Param | Env Var          |
| --------- | ---------------- |
|host       | MINIO_HOST       |
|key        | MINIO_ACCESS_KEY |
|secret     | MINIO_SECRET     |
|bucket     | MINIO_BUCKET     |
|bits       | MINIO_BITS_DIR   |
|bom        | MINIO_BOM        |

## Assumptions

- The current state of the product assumes that all file names in the BoM are relative to the `MINIO_BITS_DIR` parameter.
