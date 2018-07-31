package commands

import (
	"fmt"
)

var VERSION = "1.0.0"

type VersionCommand struct {
}

//Execute - returns the version
func (c *VersionCommand) Execute([]string) error {
	fmt.Println(VERSION)
	return nil
}
