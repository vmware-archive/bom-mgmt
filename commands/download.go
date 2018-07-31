package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
		case "pivnet-non-tile":
			DownloadPivnetNonTile(&pivnet.DownloadProductFilesCommand{
				ProductSlug:    file.ProductSlug,
				ReleaseVersion: file.Version,
				DownloadDir:    fileDir,
				AcceptEULA:     false,
				Globs:          file.Globs,
			}, bom.PivnetToken)
		case "pivnet-tile":
			os.MkdirAll(filepath.Join(fileDir, file.ProductSlug), os.ModePerm)
			DownloadPivnetTile(&pivnet.DownloadProductFilesCommand{
				ProductSlug:    file.ProductSlug,
				ReleaseVersion: file.Version,
				DownloadDir:    filepath.Join(fileDir, file.ProductSlug),
				AcceptEULA:     false,
				Globs:          file.Globs,
			}, bom.PivnetToken, bom.Iaas, file.Name)
		case "docker":
			DownloadDocker(file.ImageName, fileDir, file.Name)
		case "git":
			url := file.GitRepo + "/tarball/" + file.Branch
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

func loginPivnet(token string) {
	pivnet.Pivnet.ProfileName = "default"
	login := &pivnet.LoginCommand{
		APIToken: token,
		Host:     "https://network.pivotal.io",
	}
	err := login.Execute(make([]string, 0))
	check(err)
}

func downloadPivnet(c *pivnet.DownloadProductFilesCommand) {
	err := c.Execute(make([]string, 0))
	check(err)
}

func DownloadPivnetTile(c *pivnet.DownloadProductFilesCommand, token, iaas, fileName string) {
	loginPivnet(token)
	downloadPivnet(c)

	//stemcell stuff
	stemcellVersion := runStemcellScript(c.DownloadDir)

	downloadStemcellCmd := &pivnet.DownloadProductFilesCommand{
		ProductSlug:    "stemcells",
		ReleaseVersion: stemcellVersion,
		DownloadDir:    c.DownloadDir,
		AcceptEULA:     false,
		Globs:          []string{"*" + iaas + "*"},
	}

	err := downloadStemcellCmd.Execute(make([]string, 0))

	if err != nil {
		err := errors.New("init")
		i := 100
		for found := true; found; found = (err != nil && i > 0) {
			i--
			downloadStemcellCmd := &pivnet.DownloadProductFilesCommand{
				ProductSlug:    "stemcells",
				ReleaseVersion: stemcellVersion + "." + strconv.Itoa(i),
				DownloadDir:    c.DownloadDir,
				AcceptEULA:     false,
				Globs:          []string{"*" + iaas + "*"},
			}
			err = downloadStemcellCmd.Execute(make([]string, 0))
		}
	}
	check(err)

	os.MkdirAll(c.DownloadDir+"-tarball", os.ModePerm)
	cmd := exec.Command("tar", "-czf", filepath.Join(c.DownloadDir+"-tarball", fileName), "-C", c.DownloadDir, ".")
	cmd.Run()

}

func DownloadPivnetNonTile(c *pivnet.DownloadProductFilesCommand, token string) {
	loginPivnet(token)
	downloadPivnet(c)
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
	if _, err := os.Stat(filepath.Join(path, fileName)); os.IsExist(err) {
		return
	}
	os.MkdirAll(path+"/"+imageName+"/rootfs", os.ModePerm)
	imagePath := filepath.Join(path, imageName)
	copyMetadata(imagePath)

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
		Cmd:   []string{"/bin/sh"},
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

func runStemcellScript(path string) string {
	cmd := fmt.Sprintf("find \"%s\" -name *.pivotal | sort | head -1", path)
	fileName, err := exec.Command("sh", "-c", cmd).Output()
	check(err)

	cmd = fmt.Sprintf("unzip -l \"%s\" | grep \"metadata\" | grep 'ml$' | awk '{print $NF}'", strings.Trim(string(fileName), "\n"))
	metadata, err := exec.Command("sh", "-c", cmd).Output()
	check(err)

	cmd = fmt.Sprintf("unzip -p \"%s\" \"%s\" | grep -A5 'stemcell_criteria:' | grep 'version:' | grep -Ei '[0-9]' | awk '{print $NF}' | sed \"s/'//g;s/\\\"//g\"", strings.Trim(string(fileName), "\n"), strings.Trim(string(metadata), "\n"))
	version, err := exec.Command("sh", "-c", cmd).Output()
	check(err)

	return strings.Trim(string(version), "\n")
}

func writeMyVmwareCreds(bom model.Bom) {
	err := ioutil.WriteFile("./config.json", model.GetMyVmwareCredentials(bom), 0644)
	check(err)
}
