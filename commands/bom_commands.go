package commands

type BillOfMaterialsCommands struct {
	Version           VersionCommand           `command:"version" description:"Print version information and exit"`
	UploadBits        UploadBitsCommand        `command:"upload-bits" description:"Uploads a directory"`
	DownloadBits      DownloadBitsCommand      `command:"download-bits" description:"Downloads bits defined in the BoM"`
	GenerateResources GenerateResourcesCommand `command:"generate-resources" description:"Generates a 'resources' block that can be used in a Concourse pipeline"`
}

var BoMCommands BillOfMaterialsCommands
