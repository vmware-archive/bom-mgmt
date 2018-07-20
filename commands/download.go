package commands

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

	cli, err := docker.NewEnvClient()
	check(err)

	images, err := cli.ImageList(context.Background(), types.ImageListOptions{All: true})
	found := false
	for _, image := range images {
		for _, digest := range image.RepoDigests {
			if strings.Contains(strings.Split(digest, "@")[0], imageName) {
				found = true
			}
		}
	}

	if found == false {
		log.Println("pulling image for " + imageName)
		out, err := cli.ImagePull(context.Background(), imageName, types.ImagePullOptions{})
		check(err)
		defer out.Close()
		io.Copy(os.Stdout, out)
	}

	cid := runContainer(imageName)

	readCloser, err := cli.ContainerExport(context.Background(), cid)
	Untar(imagePath+"/rootfs", readCloser)
	file, err := os.Create(filepath.Join(path, fileName))
	defer file.Close()
	check(err)
	Tar(imagePath, file)

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

	fmt.Println(resp)
	return resp.ID
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func Untar(dst string, r io.Reader) error {

	// gzr, err := gzip.NewReader(r)
	// defer gzr.Close()
	// if err != nil {
	// 	return err
	// }

	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dst, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
		}
	}
}

func Tar(directory string, writers ...io.Writer) error {

	log.Println("Tarring " + directory)

	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(directory); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}

	mw := io.MultiWriter(writers...)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// walk path
	return filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		check(err)

		var link string
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			_, err = os.Readlink(path)
			check(err)
		}

		header, err := tar.FileInfoHeader(info, link)
		check(err)

		header.Name = filepath.Join(".", strings.TrimPrefix(path, directory))
		log.Println(header.Name)
		err = tw.WriteHeader(header)
		check(err)

		if !info.Mode().IsRegular() { //nothing more to do for non-regular
			return nil
		}

		fh, err := os.Open(path)
		check(err)
		defer fh.Close()

		_, err = io.Copy(tw, fh)
		check(err)

		return nil
	})
}

func copyMetadata(path string) {
	err := ioutil.WriteFile(path+"/metadata.json", model.DockerMetadata, 0644)
	check(err)
}

func writeMyVmwareCreds(bom model.Bom) {
	err := ioutil.WriteFile("./config.json", model.GetMyVmwareCredentials(bom), 0644)
	check(err)
}
