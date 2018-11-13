package model

import (
	"log"

	yaml "gopkg.in/yaml.v2"
)

type MinioObject struct {
	Name          string   `yaml:"name"`
	ContentType   string   `yaml:"contentType"`
	ResourceType  string   `yaml:"resourceType"`
	URL           string   `yaml:"url"`
	ProductSlug   string   `yaml:"productSlug"`
	ProductFamily string   `yaml:"productFamily"`
	Globs         []string `yaml:"globs"`
	Version       string   `yaml:"version"`
	ImageName     string   `yaml:"imageName"`
	Tag           string   `yaml:"tag"`
	GitRepo       string   `yaml:"gitRepo"`
	Branch        string   `yaml:"branch"`
	GitUser       string   `yaml:"gitUser"`
	GitPassword   string   `yaml:"gitPassword"`
}

type Bom struct {
	Bits             []MinioObject `yaml:"bits"`
	PivnetToken      string        `yaml:"pivnet_token"`
	MyVmwareUser     string        `yaml:"myvmware_user"`
	MyVmwarePassword string        `yaml:"myvmware_password"`
	Iaas             string        `yaml:"iaas"`
}

func GetBom(bomBytes []byte) Bom {
	var bom Bom
	if err := yaml.UnmarshalStrict(bomBytes, &bom); err != nil {
		log.Fatalln("unable to parse bom " + err.Error())
	}

	return bom
}
