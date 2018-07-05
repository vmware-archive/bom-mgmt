package model

import (
	"log"

	yaml "gopkg.in/yaml.v2"
)

type MinioObject struct {
	Name        string `yaml:"name"`
	Path        string
	ContentType string `yaml:"contentType"`
}

type Bom struct {
	Bits []MinioObject `yaml:"bits"`
}

func GetAllBits(path string, bomBytes []byte) []MinioObject {
	var bom Bom
	if err := yaml.UnmarshalStrict(bomBytes, &bom); err != nil {
		log.Fatalln("unable to parse bom " + err.Error())
	}

	for i, _ := range bom.Bits {
		thisBit := &bom.Bits[i]
		thisBit.Path = path + "/" + thisBit.Name
	}

	return bom.Bits
}
