package commands

type MinioUploaderCommands struct {
	Version    VersionCommand    `command:"version" description:"Print version information and exit"`
	UploadBits UploadBitsCommand `command:"upload-bits" description:"Uploads a directory"`
}

var UploaderCommands MinioUploaderCommands
