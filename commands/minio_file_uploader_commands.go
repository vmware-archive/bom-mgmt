package commands

type MinioUploaderCommands struct {
	Version      VersionCommand      `command:"version" description:"Print version information and exit"`
	UploadBits   UploadBitsCommand   `command:"upload-bits" description:"Uploads a directory"`
	DownloadBits DownloadBitsCommand `command:"download-bits" description:"Downloads bits defined in the BoM"`
}

var UploaderCommands MinioUploaderCommands
