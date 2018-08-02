package commands

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/pivotalservices/bom-mgmt/model"
	yaml "gopkg.in/yaml.v2"
)

type GenerateResourcesCommand struct {
	HostName        string `long:"host" env:"MINIO_HOST" description:"Minio Host to connect to" required:"true"`
	AccessKey       string `long:"key" env:"MINIO_ACCESS_KEY" description:"Minio Access Key used to connect to host" required:"true"`
	SecretAccessKey string `long:"secret" env:"MINIO_SECRET" description:"Minio Secret Access Key used to connect to host" required:"true"`
	Bucket          string `long:"bucket" env:"MINIO_BUCKET" description:"Minio Bucket where files will be uploaded" required:"true"`
	Bom             string `long:"bom" env:"MINIO_BOM" description:"YAML file containing information about all files to upload" required:"true"`
}

func (c *GenerateResourcesCommand) Execute([]string) error {
	dat, err := ioutil.ReadFile(c.Bom)
	check(err)

	bom := model.GetBom(dat)
	allBits := bom.Bits

	var resources []model.Resource

	for _, file := range allBits {
		r := model.Resource{
			Name: strings.Split(file.Name, ".")[0],
			Source: model.Source{
				AccessKey:       c.AccessKey,
				SecretAccessKey: c.SecretAccessKey,
				Bucket:          c.Bucket,
				Endpoint:        "http://" + c.HostName,
				RegExp:          model.GetRelativePath(file),
			},
			Type: "s3",
		}
		resources = append(resources, r)
	}

	resourcesBlock := model.ResourcesBlock{
		Resources: resources,
	}

	out, err := yaml.Marshal(resourcesBlock)
	check(err)
	fmt.Println(string(out))
	return nil
}
