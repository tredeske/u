package usftp

// sftp server integration tests
// enable with -integration
// example invokation (darwin): gofmt -w `find . -name \*.go` && (cd server_standalone/ ; go build -tags debug) && go test -tags debug github.com/pkg/sftp -integration -v -sftp /usr/libexec/sftp-server -run ServerCompareSubsystems

import (
	"flag"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestMain(m *testing.M) {
	sftpClientLocation, _ := exec.LookPath("sftp")
	testSftpClientBin = flag.String("sftp_client", sftpClientLocation, "location of the sftp client binary")

	lookSFTPServer := []string{
		"/usr/libexec/sftp-server",
		"/usr/lib/openssh/sftp-server",
		"/usr/lib/ssh/sftp-server",
		"C:\\Program Files\\Git\\usr\\lib\\ssh\\sftp-server.exe",
	}
	sftpServer, _ := exec.LookPath("sftp-server")
	if len(sftpServer) == 0 {
		for _, location := range lookSFTPServer {
			if _, err := os.Stat(location); err == nil {
				sftpServer = location
				break
			}
		}
	}
	testSftp = flag.String("sftp", sftpServer, "location of the sftp server binary")
	flag.Parse()

	os.Exit(m.Run())
}

func skipIfWindows(t testing.TB) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}
}

/*
func skipIfPlan9(t testing.TB) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping test on plan9")
	}
}
*/

var testServerImpl = flag.Bool("testserver", true, "perform integration tests against sftp package server instance")
var testIntegration = flag.Bool("integration", true, "perform integration tests against sftp server process")

// var testServerImpl = flag.Bool("testserver", false, "perform integration tests against sftp package server instance")
// var testIntegration = flag.Bool("integration", false, "perform integration tests against sftp server process")
// var testAllocator = flag.Bool("allocator", false, "perform tests using the allocator")
var testSftp *string

var testSftpClientBin *string

// var sshServerDebugStream = ioutil.Discard
// var sftpServerDebugStream = ioutil.Discard
//var sftpClientDebugStream = ioutil.Discard

const (
	GolangSFTP  = true
	OpenSSHSFTP = false
)

var (
	hostPrivateKeySigner ssh.Signer
	privKey              = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEArhp7SqFnXVZAgWREL9Ogs+miy4IU/m0vmdkoK6M97G9NX/Pj
wf8I/3/ynxmcArbt8Rc4JgkjT2uxx/NqR0yN42N1PjO5Czu0dms1PSqcKIJdeUBV
7gdrKSm9Co4d2vwfQp5mg47eG4w63pz7Drk9+VIyi9YiYH4bve7WnGDswn4ycvYZ
slV5kKnjlfCdPig+g5P7yQYud0cDWVwyA0+kxvL6H3Ip+Fu8rLDZn4/P1WlFAIuc
PAf4uEKDGGmC2URowi5eesYR7f6GN/HnBs2776laNlAVXZUmYTUfOGagwLsEkx8x
XdNqntfbs2MOOoK+myJrNtcB9pCrM0H6um19uQIDAQABAoIBABkWr9WdVKvalgkP
TdQmhu3mKRNyd1wCl+1voZ5IM9Ayac/98UAvZDiNU4Uhx52MhtVLJ0gz4Oa8+i16
IkKMAZZW6ro/8dZwkBzQbieWUFJ2Fso2PyvB3etcnGU8/Yhk9IxBDzy+BbuqhYE2
1ebVQtz+v1HvVZzaD11bYYm/Xd7Y28QREVfFen30Q/v3dv7dOteDE/RgDS8Czz7w
jMW32Q8JL5grz7zPkMK39BLXsTcSYcaasT2ParROhGJZDmbgd3l33zKCVc1zcj9B
SA47QljGd09Tys958WWHgtj2o7bp9v1Ufs4LnyKgzrB80WX1ovaSQKvd5THTLchO
kLIhUAECgYEA2doGXy9wMBmTn/hjiVvggR1aKiBwUpnB87Hn5xCMgoECVhFZlT6l
WmZe7R2klbtG1aYlw+y+uzHhoVDAJW9AUSV8qoDUwbRXvBVlp+In5wIqJ+VjfivK
zgIfzomL5NvDz37cvPmzqIeySTowEfbQyq7CUQSoDtE9H97E2wWZhDkCgYEAzJdJ
k+NSFoTkHhfD3L0xCDHpRV3gvaOeew8524fVtVUq53X8m91ng4AX1r74dCUYwwiF
gqTtSSJfx2iH1xKnNq28M9uKg7wOrCKrRqNPnYUO3LehZEC7rwUr26z4iJDHjjoB
uBcS7nw0LJ+0Zeg1IF+aIdZGV3MrAKnrzWPixYECgYBsffX6ZWebrMEmQ89eUtFF
u9ZxcGI/4K8ErC7vlgBD5ffB4TYZ627xzFWuBLs4jmHCeNIJ9tct5rOVYN+wRO1k
/CRPzYUnSqb+1jEgILL6istvvv+DkE+ZtNkeRMXUndWwel94BWsBnUKe0UmrSJ3G
sq23J3iCmJW2T3z+DpXbkQKBgQCK+LUVDNPE0i42NsRnm+fDfkvLP7Kafpr3Umdl
tMY474o+QYn+wg0/aPJIf9463rwMNyyhirBX/k57IIktUdFdtfPicd2MEGETElWv
nN1GzYxD50Rs2f/jKisZhEwqT9YNyV9DkgDdGGdEbJNYqbv0qpwDIg8T9foe8E1p
bdErgQKBgAt290I3L316cdxIQTkJh1DlScN/unFffITwu127WMr28Jt3mq3cZpuM
Aecey/eEKCj+Rlas5NDYKsB18QIuAw+qqWyq0LAKLiAvP1965Rkc4PLScl3MgJtO
QYa37FK0p8NcDeUuF86zXBVutwS5nJLchHhKfd590ks57OROtm29
-----END RSA PRIVATE KEY-----
`)
)

func init() {
	var err error
	hostPrivateKeySigner, err = ssh.ParsePrivateKey(privKey)
	if err != nil {
		panic(err)
	}
}
