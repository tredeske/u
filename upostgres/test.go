package upostgres

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	osu "os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/uexec"
	"github.com/tredeske/u/uio"
	"github.com/tredeske/u/ulog"
)

//
// for testing to work, postgres must be installed on your system and the
// commands must be in your path.
//
// postres likes to put things in strange places, such as:
//
//    /usr/pgsql-VERSION/bin
//

//
// sets up a test area for testing
//
func CreateTestArea(name string) (dir string, err error) {

	dir, err = filepath.Abs("./.test-area-" + name)
	if err != nil {
		err = uerr.Chainf(err, "Unable to setup test area")
		return
	}
	uio.FileRemoveAll(dir)
	return
}

//
// sets up a test area for testing
//
func CreateTestAreaTmp(name string) (dir string, err error) {

	dir, err = filepath.Abs("/tmp/.test-area-" + name)
	if err != nil {
		err = uerr.Chainf(err, "Unable to setup test area")
		return
	}
	uio.FileRemoveAll(dir)
	return
}

func GetTestProps(tname string) (user string, err error) {

	userInfo, err := osu.Current()
	if nil != err {
		err = uerr.Chainf(err, "cannot get username")
		return
	}
	user = userInfo.Username

	/*
		p, err := props.LoadTestProps()
		if err != nil {
			err = uerr.Chainf(err, "loading props")
			return
		}

		giteaP := p.Props("gitea")
		if 0 == len(giteaP) {
			err = uerr.Chainf(err, "unable to find 'gitea' section")
			return
		}

		user = giteaP.String("user", user)
		if 0 == len(user) {
			err = uerr.Chainf(err, "unable to find 'gitea.user'")
			return
		}

		url = giteaP.String("url", "")
		if 0 == len(url) {
			err = uerr.Chainf(err, "unable to find 'gitea.url'")
			return
		}

		ulog.Printf("gitea user=%s, group=%s, url=%s", user, group, url)
	*/
	ulog.Printf("postgres user=%s", user)
	return
}

type TestServer struct {
	Dir      string // where to place the DB
	Database string // name of DB - must be lowercase
	Port     int    // port DB should listen on
	pgctl    string //
}

func (this *TestServer) Start() (err error) {

	if 0 >= this.Port {
		this.Port = 5433
	}
	if 0 == len(this.Dir) {
		err = errors.New("Dir not specified")
		return
	} else if 0 == len(this.Database) {
		err = errors.New("Database not specified")
		return
	} else if this.Database != strings.ToLower(this.Database) {
		err = errors.New("Database name must be all lower case")
		return
	}

	initdb, err := exec.LookPath("initdb")
	if err != nil {
		err = uerr.Chainf(err, "postgres initdb not installed.  check /usr/pgsql-VERSION/bin and make sure your PATH is set up")
		return
	}
	pg, err := exec.LookPath("postgres")
	if err != nil {
		err = uerr.Chainf(err, "postgres not installed.  check /usr/pgsql-VERSION/bin and make sure your PATH is set up")
		return
	}
	this.pgctl, err = exec.LookPath("pg_ctl")
	if err != nil {
		err = uerr.Chainf(err, "postgres pg_ctl not installed.  check /usr/pgsql-VERSION/bin and make sure your PATH is set up")
		return
	}

	//
	// init db disk storage
	//
	err = uexec.Sh(initdb, "-D", this.Dir)
	if err != nil {
		err = uerr.Chainf(err, "unable to initialize postgres db")
		return
	}

	//
	// create database
	//
	ulog.Printf("test postgres: creating DB %s in %s", this.Database, this.Dir)
	pgc := uexec.NewChild(pg, "--single", "-D", this.Dir, "postgres")
	err = pgc.AddPipe(uexec.STDIN)
	if err != nil {
		return
	}
	go func() {
		wpipe := pgc.ParentIo[uexec.STDIN]
		_, err := wpipe.WriteString("CREATE DATABASE " + this.Database + ";\n/q\n")
		if err != nil {
			ulog.Errorf("Unable to write db create: %s", err)
		}
		err = wpipe.Close()
		if err != nil {
			ulog.Errorf("Unable to close after db create: %s", err)
		}
	}()
	var bb bytes.Buffer
	err = pgc.ShToBuff(&bb)
	if err != nil {
		return
	} else if bytes.Contains(bb.Bytes(), []byte("ERROR")) {
		err = fmt.Errorf("Unable to create db: %s", bb.String())
		return
	}
	ulog.Debugf("Response to DB create: '%s'", bb.String())

	//
	// start it
	//

	//
	// these opts set the listen port and the dir where lock files go
	// man postgres
	//
	ulog.Printf("test postgres: starting server on port %d", this.Port)
	opts := "-p " + strconv.Itoa(this.Port) + " -k " + this.Dir
	err = uexec.Sh(this.pgctl, "-D", this.Dir, "-w", "-l", this.Dir+"/logfile", "-o", opts, "start")
	if err != nil {
		err = uerr.Chainf(err, "unable to start postgres db")
		return
	}

	return
}

func (this *TestServer) Stop() (err error) {

	err = uexec.Sh(this.pgctl, "-D", this.Dir, "-l", "logfile", "stop")
	if err != nil {
		err = uerr.Chainf(err, "unable to stop postgres db")
	}
	return
}
