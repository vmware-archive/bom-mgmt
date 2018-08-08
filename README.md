# Bill of Materials Management (bom-mgmt)

Go automation for downloading and/or uploading(to a Minio server) a set of files from a Bill of Materials (BoM).

## Bill of Materials

This product utilizes a YAML file structure that lays out a bill of materials like this:
```
pivnet_token: abcdefghijklmnop
myvmware_user: user@foo.com
myvmware_password: password1!
iaas: vsphere
bits:
- name: abc.txt
  contentType: text/plain
  resourceType: file
  url: https://foo.com/download/abc.txt
- name: repo.zip
  contentType: application/zip
  resourceType: git
  branch: master
  gitRepo: https://github.com/myuser/repo
- name: ubuntu.tgz
  contentType: application/gzip
  resourceType: docker
  imageName: ubuntu
- name: vmware_tool.ova
  contentType: application/vmware
  resourceType: vmware
  productSlug: vmware_tool-1.1.1.ova
  group: "Set of Tools"
- name: pivotal-container-service-1.1.2.tgz
  contentType: application/gzip
  productSlug: pivotal-container-service
  version: 1.1.2
  globs: ["*.pivotal"]
  resourceType: pivnet-tile
- name: pcf-ops-manager-2.2.1.ova
  contentType: application/vmware
  productSlug: ops-manager
  version: 2.2.1
  globs: ["*vsphere*"]
  resourceType: pivnet-non-tile
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

## Resource Types

| Type           | Additional BoM Params        | Requirements                                | Output                           |
| -------------- | ---------------------------- | ------------------------------------------- | -------------------------------- |
|file            | url                          |                                             | the specified file               |
|git             | branch, gitRepo              |                                             | .tgz of the repo                 |
|docker          | imageName, tag (optional)    | Uses docker environment from machine        | .tgz of the image                |
|vmware          | productSlug, group           | Need to provide myvmware credentials in BoM | the specified file               |
|pivnet-tile     | productSlug, globs, version  | Need to provide pivnetToken in Bom          | .tgz of tile and needed stemcell |
|pivnet-non-tile | productSlug, globs, version  | Need to provide pivnetToken in Bom          | the specified file               |

### File Paths
| Type           | Path                                                                    |
| -------------- | ----------------------------------------------------------------------- |
|file            | {MINIO_BITS_DIR}/resources/file/{filename}                              |
|git             | {MINIO_BITS_DIR}/resources/git/{filename}                               |
|docker          | {MINIO_BITS_DIR}/resources/docker/{filename}                            |
|vmware          | {MINIO_BITS_DIR}/resources/vmware/{filename}                            |
|pivnet-tile     | {MINIO_BITS_DIR}/resources/pivnet-tile/{productSlug}-tarball/{filename} |
|pivnet-non-tile | {MINIO_BITS_DIR}/resources/pivnet-non-tile/{filename}                   |

## Generating Resources Block
This command will use the contents of the BoM and Minio server information to generate a `resources` block that can be used in a Concourse pipeline. Each resource will be of type `s3` and point to the Minio server and bucket provided using the paths above.

```
export MINIO_HOST="localhost:9000"
export MINIO_ACCESS_KEY="key"
export MINIO_SECRET="secretsquirrel"

bom-mgmt generate-resources --bom "bom.yml" --bucket "bar"
```

## Assumptions

- The Docker CLI is present on the system running this script
