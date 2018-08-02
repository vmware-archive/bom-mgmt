package model

type ResourcesBlock struct {
	Resources []Resource `yaml:"resources"`
}

type Resource struct {
	Name   string `yaml:"name"`
	Source Source `yaml:"source"`
	Type   string `yaml:"type"`
}

type Source struct {
	AccessKey       string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	Bucket          string `yaml:"bucket"`
	Endpoint        string `yaml:"endpoint"`
	RegExp          string `yaml:"regexp"`
}
