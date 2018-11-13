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

	// Git related
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
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
			os.MkdirAll(filepath.Join(fileDir, file.ProductSlug), os.ModePerm)
			DownloadPivnetNonTile(&pivnet.DownloadProductFilesCommand{
				ProductSlug:    file.ProductSlug,
				ReleaseVersion: file.Version,
				DownloadDir:    filepath.Join(fileDir, file.ProductSlug),
				AcceptEULA:     false,
				Globs:          file.Globs,
			}, bom.PivnetToken, file.Name)
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
		case "gitclone":
			GitClone(fileDir, filePath, file)
		case "vmware":
			writeMyVmwareCreds(bom, fileDir)
			DownloadVMWare(file.Name, file.ProductSlug, file.ProductFamily, fileDir)
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

func GitClone(fileDir string, filePath string, file model.MinioObject) {

	var auth *githttp.BasicAuth

	repoName := strings.Split(file.GitRepo, "/")[len(strings.Split(file.GitRepo, "/"))-1]
	repoDir := filepath.Join(fileDir, repoName)
	os.MkdirAll(repoDir, os.ModePerm)

	log.Println("checking out", file.GitRepo, "as", repoDir)

	// add credentials if passed in
	if file.GitUser != "" && file.GitPassword != "" {
		log.Println("with user", file.GitUser)
		auth = &githttp.BasicAuth{
			Username: file.GitUser,
			Password: file.GitPassword,
		}
	}

	// // clean up left-behind files due to past failure
	// cmd := exec.Command("rm", "-rf", filepath.Join(fileDir, file.Name))
	// cmd.Run()

	// clone the repository
	_, err := git.PlainClone(repoDir, false, &git.CloneOptions{
		Auth:          auth,
		URL:           file.GitRepo,
		Progress:      os.Stdout,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", file.Branch)),
		SingleBranch:  true,
	})
	check(err)

	// wipe out git information
	cmd := exec.Command("rm", "-rf", filepath.Join(repoDir, ".git"))
	err = cmd.Run()
	check(err)

	cmd = exec.Command("tar", "-czf", filePath, "-C", repoDir, ".")
	err = cmd.Run()
	check(err)

	cmd = exec.Command("rm", "-rf", repoDir)
	err = cmd.Run()
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
	stemcellOS, stemcellVersion := runStemcellScript(c.DownloadDir)

	productSlug := "stemcells"
	if stemcellOS == "ubuntu-xenial" {
		productSlug = "stemcells-ubuntu-xenial"
	}

	downloadStemcellCmd := &pivnet.DownloadProductFilesCommand{
		ProductSlug:    productSlug,
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

	// Cleanup empty stemcells or other downloads
	cleanup_empty_files_cmd := exec.Command(fmt.Sprintf("find %s -size 0 -exec rm {} \\;", c.DownloadDir))
	cleanup_empty_files_cmd.Run()

	cmd := exec.Command("tar", "-czf", filepath.Join(c.DownloadDir+"-tarball", fileName), "-C", c.DownloadDir, ".")
	cmd.Run()

	cmd = exec.Command("rm", "-rf", c.DownloadDir)
	cmd.Run()

}

func DownloadPivnetNonTile(c *pivnet.DownloadProductFilesCommand, token, filename string) {
	loginPivnet(token)
	downloadPivnet(c)
	cmd := fmt.Sprintf("find %s -name %s | xargs -I '{}' mv {} %s", filepath.Join(c.DownloadDir, ".."), c.Globs[0], filepath.Join(c.DownloadDir, "..", filename))
	shell.RunCommand(cmd)
	cmd = fmt.Sprintf("rm -rf %s", c.DownloadDir)
	shell.RunCommand(cmd)
}

func DownloadVMWare(fileName, slug, productFamily, fileDir string) {
	const imageName = "apnex/vmw-cli"
	cmd := fmt.Sprintf("docker pull %s", imageName)
	shell.RunCommand(cmd)

	// Use absolute path for specifying volumes
	mount_path := fileDir
	if (! strings.HasPrefix(mount_path, "/")) {
		mount_path = fmt.Sprintf("${PWD}/%s", mount_path)
	}

	apex_vmw_cli_docker_start_cmd := fmt.Sprintf("docker run -e VMWUSER -e VMWPASS -v %s:/files %s ",
																										mount_path,
																										imageName)

	if _, err := os.Stat(fmt.Sprintf("%s/fileIndex.json", fileDir)); os.IsNotExist(err) {
		fmt.Printf("Unable to find cached index file: %s/fileIndex.json!!\n", mount_path)
		fmt.Printf("Building indexes before proceeding with download of product file: %s\n", slug)

		// No cached indexes exists
		cmd = fmt.Sprintf("%s list", apex_vmw_cli_docker_start_cmd)
		shell.RunCommand(cmd)

		// Run index against product family vmware-nsx-t-data-center & vmware-pivotal-container-service by default
		nsx_t_product_family := "vmware-nsx-t-data-center"
		cmd = fmt.Sprintf("%s index %s", apex_vmw_cli_docker_start_cmd, nsx_t_product_family)
		shell.RunCommand(cmd)

		pks_product_family := "vmware-pivotal-container-service"
		cmd = fmt.Sprintf("%s index %s", apex_vmw_cli_docker_start_cmd, pks_product_family)
		shell.RunCommand(cmd)

		vmware_vsphere_product_family := "vmware-vsphere"
		fmt.Println("WARNING!! Downloading of the vmware-vsphere product family can take a long time, upwards of 5 minutes!!")
		cmd = fmt.Sprintf("%s index %s", apex_vmw_cli_docker_start_cmd, vmware_vsphere_product_family)
		shell.RunCommand(cmd)

		if (productFamily != "" &&  productFamily != "vmware-vsphere") {
			cmd = fmt.Sprintf("%s index %s", apex_vmw_cli_docker_start_cmd, productFamily)
			shell.RunCommand(cmd)
		}

	}

	// Run find at top level
	fmt.Println("\nJust running a generic find for available NSX products!!")
	cmd = fmt.Sprintf("%s find fileName:nsx*", apex_vmw_cli_docker_start_cmd)
	shell.RunCommand(cmd)

	// Run find for matching product
	fmt.Printf("\nJust running a match for specified Product: %s!!\n", slug)
	cmd = fmt.Sprintf("%s find fileName:%s", apex_vmw_cli_docker_start_cmd, slug)
	shell.RunCommand(cmd)

	// Default handling of product slug
	cmd = fmt.Sprintf("%s get %s", apex_vmw_cli_docker_start_cmd, slug)
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

func runStemcellScript(path string) (string, string) {
	cmd := fmt.Sprintf("find \"%s\" -name *.pivotal | sort | head -1", path)
	fileName, err := exec.Command("sh", "-c", cmd).Output()
	check(err)

	cmd = fmt.Sprintf("unzip -l \"%s\" | grep \"metadata\" | grep 'ml$' | awk '{print $NF}'", strings.Trim(string(fileName), "\n"))
	metadata, err := exec.Command("sh", "-c", cmd).Output()
	check(err)

	cmd = fmt.Sprintf("unzip -p \"%s\" \"%s\" | grep -A5 'stemcell_criteria:' | grep 'version:' | grep -Ei '[0-9]' | awk '{print $NF}' | sed \"s/'//g;s/\\\"//g\"", strings.Trim(string(fileName), "\n"), strings.Trim(string(metadata), "\n"))
	version, err := exec.Command("sh", "-c", cmd).Output()
	check(err)

	cmd = fmt.Sprintf("unzip -p \"%s\" \"%s\" | grep -A5 'stemcell_criteria:' | grep 'os:' | awk '{print $NF}' ", strings.Trim(string(fileName), "\n"), strings.Trim(string(metadata), "\n"))
	stemcellOS, err := exec.Command("sh", "-c", cmd).Output()
	check(err)

	return strings.Trim(string(stemcellOS), "\n"), strings.Trim(string(version), "\n")
}

func writeMyVmwareCreds(bom model.Bom, fileDir string) {
	os.Setenv("VMWUSER", bom.MyVmwareUser)
	os.Setenv("VMWPASS", bom.MyVmwarePassword)
}
