package model

import "path/filepath"

func GetRelativePath(file MinioObject) string {
	switch resourceType := file.ResourceType; resourceType {
	case "pivnet-tile":
		return filepath.Join("resources", resourceType, file.ProductSlug+"-tarball", file.Name)
	default:
		return filepath.Join("resources", resourceType, file.Name)
	}
}
