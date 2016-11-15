package uconfig

import "os/user"

func UserInfo() (userName string, homeD string) {
	defer func() { recover() }() // avoid panic when running in small docker container
	if currentUser, err := user.Current(); nil == err {
		userName = currentUser.Username
		homeD = currentUser.HomeDir
	}
	return
}
