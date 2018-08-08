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

	"docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"github.com/pivotalservices/bom-mgmt/model"
	"github.com/pivotalservices/bom-mgmt/shell"

	pivnet "github.com/pivotal-cf/pivnet-cli/commands"
)

type DownloadBitsCommand struct {
	Bom     string `long:"bom" env:"MINIO_BOM" description:"YAML file containing information about all files to upload" required:"true"`
	BitsDir string `long:"bits" env:"MINIO_BITS_DIR" description:"Minio Secret Access Key used to connect to host" required:"true"`
}

func (c *DownloadBitsCommand) Execute([]string) error {
	dat, err := ioutil.ReadFile(c.Bom)
	check(err)

	bom := model.GetBom(dat)
	allBits := bom.Bits

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
			DownloadDocker(file.ImageName, file.Tag, fileDir, file.Name)
		case "git":
			DownloadGit(filePath, file.GitRepo, file.Branch, fileDir)
		case "vmware":
			writeMyVmwareCreds(bom, fileDir)
			DownloadVMWare(file.Name, file.Group, file.ProductSlug, fileDir)
		default:
			log.Fatalln("Resource Type '" + resourceType + "' is not recognized")
		}
	}

	cmd := exec.Command("ls", "-la", "-R", filepath.Join(c.BitsDir, "resources"))
	cmd.Run()

	return nil
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(filePath, url string) {
	log.Println("Downloading " + filePath + " from " + url)

	// Create the file
	out, err := os.Create(filePath)
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

func DownloadGit(filePath, repo, branch, fileDir string) {
	url := repo + "/tarball/" + branch
	DownloadFile(filePath, url)

	repoName := strings.Split(repo, "/")[len(strings.Split(repo, "/"))-1]

	os.MkdirAll(filepath.Join(fileDir, repoName), os.ModePerm)

	cmd := exec.Command("tar", "xzf", filePath, "--strip=1", "-C", filepath.Join(fileDir, repoName))
	cmd.Run()

	cmd = exec.Command("rm", filePath)
	cmd.Run()

	cmd = exec.Command("tar", "-czf", filePath, "-C", filepath.Join(fileDir, repoName), ".")
	cmd.Run()

	cmd = exec.Command("rm", "-rf", filepath.Join(fileDir, repoName))
	cmd.Run()

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
		check(err)
	}

	os.MkdirAll(c.DownloadDir+"-tarball", os.ModePerm)
	cmd := exec.Command("tar", "-czf", filepath.Join(c.DownloadDir+"-tarball", fileName), "-C", c.DownloadDir, ".")
	cmd.Run()

	cmd = exec.Command("rm", "-rf", c.DownloadDir)
	cmd.Run()

}

func DownloadPivnetNonTile(c *pivnet.DownloadProductFilesCommand, token string) {
	loginPivnet(token)
	downloadPivnet(c)
}

func DownloadVMWare(fileName, group, slug, fileDir string) {
	const imageName = "apnex/myvmw"
	cmd := fmt.Sprintf("docker pull %s", imageName)
	shell.RunCommand(cmd)
	cmd = fmt.Sprintf("docker run -v %s:/vmwfiles %s", fileDir, imageName)
	shell.RunCommand(cmd)
	cmd = fmt.Sprintf("docker run -v %s:/vmwfiles %s \"%s\"", fileDir, imageName, group)
	shell.RunCommand(cmd)
	cmd = fmt.Sprintf("docker run -v %s:/vmwfiles %s get %s", fileDir, imageName, slug)
	shell.RunCommand(cmd)
	cmd = fmt.Sprintf("mv %s %s", filepath.Join(fileDir, slug), filepath.Join(fileDir, fileName))
	shell.RunCommand(cmd)
}

func DownloadDocker(imageName, tag, path, fileName string) {
	os.MkdirAll(path+"/"+imageName+"/rootfs", os.ModePerm)
	imagePath := filepath.Join(path, imageName)
	copyMetadata(imagePath)

	var cid string
	if tag == "" {
		cid = runContainer(imageName)
	} else {
		cid = runContainer(imageName + ":" + tag)
	}

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
	cmd = exec.Command("tar", "-xvf", filepath.Join(imagePath, "rootfs.tar"), "-C", filepath.Join(imagePath, "rootfs/"), "--exclude=dev/*")
	cmd.Run()

	// remove the tar file
	cmd = exec.Command("rm", filepath.Join(imagePath, "rootfs.tar"))
	cmd.Run()
	//tar the metadata.json and rootfs folder together
	cmd = exec.Command("tar", "-czf", filepath.Join(path, fileName), "-C", imagePath, ".")
	cmd.Run()

	cmd = exec.Command("rm", "-rf", imagePath)
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

func writeMyVmwareCreds(bom model.Bom, fileDir string) {
	err := ioutil.WriteFile(filepath.Join(fileDir, "config.json"), model.GetMyVmwareCredentials(bom), 0644)
	check(err)
}
