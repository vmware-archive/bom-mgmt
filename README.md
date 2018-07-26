# Bill of Materials Management (bom-mgmt)

Go automation for downloading and/or uploading(to a Minio server) a set of files from a Bill of Materials (BoM).

## Bill of Materials

This product utilizes a YAML file structure that lays out a bill of materials like this:
```
pivnet_token: abcdefghijklmnop
myvmware_user: user@foo.com
myvmware_password: password1!
bits:
- name: repo.zip
  contentType: application/zip
  resourceType: git
  branch: master
  gitRepo: https://github.com/myuser/repo
- name: abc.txt
  contentType: text/plain
  resourceType: file
  url: https://foo.com/download/abc.txt
- name: vmware_tool.ova
  contentType: application/vmware
  resourceType: vmware
- name: ubuntu.tgz
  contentType: application/gzip
  resourceType: docker
  imageName: ubuntu
- name: bbr-1.2.4.tar
  contentType: application/tar
  resourceType: pivnet
  productSlug: p-bosh-backup-and-restore
  version: 1.2.4
```

## Build from the source

`bom-mgmt` is written in [Go](https://golang.org/).
To build the binary yourself, follow these steps:

* Install `Go`.
* Install [Dep](https://github.com/golang/dep), a dependency management tool for Go.
* Clone the repo:
  - `mkdir -p $(go env GOPATH)/src/github.com/pivotalservices`
  - `cd $(go env GOPATH)/src/github.com/pivotalservices`
  - `git clone git@github.com:pivotalservices/bom-mgmt.git`
* Install dependencies:
  - `cd bom-mgmt`
  - `dep ensure`
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

### Parameters

All of the necessary parameters can either be passed on the command line or as environment variables.

| CLI Param | Env Var          |
| --------- | ---------------- |
|host       | MINIO_HOST       |
|key        | MINIO_ACCESS_KEY |
|secret     | MINIO_SECRET     |
|bucket     | MINIO_BUCKET     |
|bits       | MINIO_BITS_DIR   |
|bom        | MINIO_BOM        |

## Downloading
```
bom-mgmt download-bits --bits "/Users/foo/test-bits" --bom "bom.yml"
```

### Parameters

All of the necessary parameters can either be passed on the command line or as environment variables.

| CLI Param | Env Var          |
| --------- | ---------------- |
|bits       | MINIO_BITS_DIR   |
|bom        | MINIO_BOM        |

### Resource Types

| Type       | Additional BoM Params | Requirements                                | Output             |
| ---------- | --------------------- | ------------------------------------------- | ------------------ |
|file        | url                   |                                             | the specified file |
|git         | branch, gitRepo       |                                             | .zip of the repo   |
|docker      | imageName             | Uses docker environment from machine        | .tgz of the image  |
|vmware      |                       | Need to provide myvmware credentials in BoM | the specified file |
|pivnet      | productSlug, version  | Need to provide pivnetToken in Bom          | the specified file |

## Assumptions

- The current state of the product assumes that all file names in the BoM are relative to the `MINIO_BITS_DIR` parameter.
