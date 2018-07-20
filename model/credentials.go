package model

import "encoding/json"

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func GetMyVmwareCredentials(bom Bom) []byte {
	creds, _ := json.Marshal(&Credentials{
		Username: bom.MyVmwareUser,
		Password: bom.MyVmwarePassword,
	})
	return creds
}
