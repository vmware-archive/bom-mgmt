package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"github.com/pivotalservices/bom-mgmt/model"

	pivnet "github.com/pivotal-cf/pivnet-cli/commands"
)

type DownloadBitsCommand struct {
	Bom     string `long:"bom" env:"MINIO_BOM" description:"YAML file containing information about all files to upload" required:"true"`
	BitsDir string `long:"bits" env:"MINIO_BITS_DIR" description:"Minio Secret Access Key used to connect to host" required:"true"`
}

func (c *DownloadBitsCommand) Execute([]string) error {
	dat, err := ioutil.ReadFile(c.Bom)
	check(err)

	bom := model.GetBom(c.BitsDir, dat)
	allBits := bom.Bits

	writeMyVmwareCreds(bom)

	for _, file := range allBits {
		fileDir := filepath.Join(c.BitsDir, "resources", file.ResourceType)
		os.MkdirAll(fileDir, os.ModePerm)
		filePath := filepath.Join(fileDir, file.Name)
		switch resourceType := file.ResourceType; resourceType {
		case "file":
			DownloadFile(filePath, file.URL)
		case "pivnet-non-tile", "pivnet-tile":
			DownloadPivnet(&pivnet.DownloadProductFilesCommand{
				ProductSlug:    file.ProductSlug,
				ReleaseVersion: file.Version,
				DownloadDir:    fileDir,
				AcceptEULA:     true,
				Globs:          []string{"*"},
			}, bom.PivnetToken)
		case "docker":
			DownloadDocker(file.ImageName, fileDir, file.Name)
		case "git":
			url := file.GitRepo + "/archive/" + file.Branch + ".zip"
			DownloadFile(filePath, url)
		case "vmware":
			DownloadVMWare(file.Name, fileDir)
		default:
			log.Fatalln("Resource Type '" + resourceType + "' is not recognized")
		}
	}

	return nil
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(filepath, url string) {
	log.Println("Downloading " + filepath + " from " + url)

	// Create the file
	out, err := os.Create(filepath)
	check(err)
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	check(err)

	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	check(err)
}

func DownloadPivnet(c *pivnet.DownloadProductFilesCommand, token string) {
	pivnet.Pivnet.ProfileName = "default"
	login := &pivnet.LoginCommand{
		APIToken: token,
		Host:     "https://network.pivotal.io",
	}
	err := login.Execute(make([]string, 0))
	check(err)

	err = c.Execute(make([]string, 0))
	check(err)
}

func DownloadVMWare(fileName, fileDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cli, err := docker.NewEnvClient()
	check(err)

	const imageName = "apnex/myvmw"
	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	check(err)
	io.Copy(os.Stdout, out)
	log.Println("fileDir:" + fileDir)
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Cmd:   []string{"get " + fileName},
	}, &container.HostConfig{
		Binds: []string{fileDir + ":/vmwfiles"},
	}, nil, "")
	check(err)

	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	check(err)
	waitC, errC := cli.ContainerWait(ctx, resp.ID, "not-running")
	select {
	case <-waitC:
		log.Println("done")
	case <-errC:
		log.Println("Download exceeded timeout, but still attempting to finish in background")
	}

	fmt.Println(resp)
}

func DownloadDocker(imageName, path, fileName string) {
	os.MkdirAll(path+"/"+imageName+"/rootfs", os.ModePerm)
	copyMetadata(path + "/" + imageName)
	imagePath := filepath.Join(path, imageName)

	cid := runContainer(imageName)

	cmd := exec.Command("docker", "export", cid)

	file, err := os.Create(filepath.Join(imagePath, "rootfs.tar"))
	defer file.Close()
	check(err)

	stdoutPipe, err := cmd.StdoutPipe()
	check(err)

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	err = cmd.Start()
	check(err)

	go io.Copy(writer, stdoutPipe)
	cmd.Wait()

	//Untar
	cmd = exec.Command("tar", "-xvf", filepath.Join(imagePath, "rootfs.tar"), "-C", filepath.Join(imagePath, "rootfs/"), "--exclude='dev/*'")
	cmd.Run()
	//remove the tar file
	cmd = exec.Command("rm", filepath.Join(imagePath, "rootfs.tar"))
	cmd.Run()
	//tar the metadata.json and rootfs folder together
	cmd = exec.Command("tar", "-czf", filepath.Join(path, fileName), "-C", imagePath, ".")
	cmd.Run()

}

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func runContainer(imageName string) string {
	ctx := context.Background()
	cli, err := docker.NewEnvClient()
	check(err)

	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	check(err)
	io.Copy(os.Stdout, out)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
	}, nil, nil, "")
	check(err)

	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	check(err)

	return resp.ID
}

func copyMetadata(path string) {
	err := ioutil.WriteFile(path+"/metadata.json", model.DockerMetadata, 0644)
	check(err)
}

func writeMyVmwareCreds(bom model.Bom) {
	err := ioutil.WriteFile("./config.json", model.GetMyVmwareCredentials(bom), 0644)
	check(err)
}
