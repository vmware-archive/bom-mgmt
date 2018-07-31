package commands

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	minio "github.com/minio/minio-go"
	"github.com/pivotalservices/bom-mgmt/model"
)

type UploadBitsCommand struct {
	HostName        string `long:"host" env:"MINIO_HOST" description:"Minio Host to connect to" required:"true"`
	AccessKey       string `long:"key" env:"MINIO_ACCESS_KEY" description:"Minio Access Key used to connect to host" required:"true"`
	SecretAccessKey string `long:"secret" env:"MINIO_SECRET" description:"Minio Secret Access Key used to connect to host" required:"true"`
	Bucket          string `long:"bucket" env:"MINIO_BUCKET" description:"Minio Bucket where files will be uploaded" required:"true"`
	BitsDir         string `long:"bits" env:"MINIO_BITS_DIR" description:"Minio Secret Access Key used to connect to host" required:"true"`
	Bom             string `long:"bom" env:"MINIO_BOM" description:"YAML file containing information about all files to upload" required:"true"`
}

const BUCKET_LOCATION = ""

func (c *UploadBitsCommand) Execute([]string) error {
	// Initialize minio client object.
	minioClient, err := minio.New(c.HostName, c.AccessKey, c.SecretAccessKey, false)
	if err != nil {
		log.Fatalln(err)
		return err
	}

	dat, err := ioutil.ReadFile(c.Bom)
	if err != nil {
		log.Fatalln(err)
		return err
	}

	bom := model.GetBom(c.BitsDir, dat)
	allBits := bom.Bits

	if err = validateBits(allBits, c.BitsDir); err != nil {
		log.Fatalln(err)
		return err
	}

	err = minioClient.MakeBucket(c.Bucket, BUCKET_LOCATION)
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, err := minioClient.BucketExists(c.Bucket)
		if err == nil && exists {
			log.Printf("We already own %s\n", c.Bucket)
		} else {
			log.Fatalln(err)
			return err
		}
	}
	log.Printf("Successfully created %s\n", c.Bucket)

	for _, file := range allBits {
		filePath := filepath.Join(c.BitsDir, "resources", file.ResourceType)
		bucketPath := filepath.Join("resources", file.ResourceType)
		if file.ResourceType == "pivnet-tile" {
			filePath = filepath.Join(filePath, file.ProductSlug+"-tarball")
			bucketPath = filepath.Join(bucketPath, file.ProductSlug+"-tarball")
		}
		n, err := minioClient.FPutObject(c.Bucket, filepath.Join(bucketPath, file.Name), filepath.Join(filePath, file.Name), minio.PutObjectOptions{ContentType: file.ContentType})
		if err != nil {
			log.Fatalln(err)
			return err
		}
		log.Printf("Successfully uploaded %s of size %d\n", file.Name, n)
	}

	return nil
}

func validateBits(allBits []model.MinioObject, bitsDir string) error {
	for _, file := range allBits {
		filePath := filepath.Join(bitsDir, "resources", file.ResourceType)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return errors.New(filePath + " is not present")
		}
	}
	return nil
}
