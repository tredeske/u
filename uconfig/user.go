package uconfig

import "os/user"

func UserInfo() (userName string, homeD string) {
	defer func() { recover() }() // avoid panic when running in small docker container
	currentUser, err := user.Current()
	if nil == err {
		userName = currentUser.Username
		homeD = currentUser.HomeDir
	}
	return
}
