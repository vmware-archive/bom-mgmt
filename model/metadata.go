package model

import "encoding/json"

type Metadata struct {
	User string   `json:"user"`
	Env  []string `json:"env"`
}

var DockerMetadata []byte

func init() {
	meta, _ := json.Marshal(&Metadata{
		User: "root",
		Env:  []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C", "HOME=/root"},
	})

	DockerMetadata = meta
}
