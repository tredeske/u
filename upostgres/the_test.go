package upostgres

import (
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/tredeske/u/uio"
)

//
// - spin up a postgres server
// - connect to it
// - disconnect
// - shutdown the postgres server
//
func TestStartStop(t *testing.T) {

	//
	// skip?
	//

	_, err := exec.LookPath("initdb")
	if err != nil {
		t.Skip("Skipping test because postgres initdb not installed")
	}

	//
	//
	//

	testD, err := CreateTestAreaTmp(t.Name())
	if err != nil {
		t.Fatalf("unable to create test area: %s", err)
	}
	server := TestServer{
		Dir:      path.Join(testD, "db"),
		Port:     5433,
		Database: strings.ToLower(t.Name()),
	}

	defer func() {
		server.Stop()
		uio.FileRemoveAll(testD)
	}()

	err = server.Start()
	if err != nil {
		t.Fatalf("unable to start server: %s", err)
	}

	conn := Connector{
		Database: server.Database,
		Port:     server.Port,
		SslMode:  "disable",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS test( id TEXT, descr TEXT );`,
		},
	}

	db, err := conn.Connect()
	if err != nil {
		t.Fatalf("Unable to connect: %s", err)
	}
	err = db.Close()
	if err != nil {
		t.Fatalf("Unable to close conn to db: %s", err)
	}

	err = server.Stop()
	if err != nil {
		t.Fatalf("unable to stop server: %s", err)
	}
}
