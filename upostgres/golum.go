package upostgres

import (
	"fmt"

	_ "github.com/lib/pq"
	"github.com/tredeske/u/uconfig"
)

//
// golum help for creating postgres connector
//
func ShowConnector(name, descr string, help *uconfig.Help) {

	p := help
	if 0 != len(name) {
		if 0 == len(descr) {
			descr = "Postgres client"
		}
		p = help.Init(name, descr)
	}
	p.NewItem("dbName", "string", "name of db to connect to")
	p.NewItem("dbUser", "string", "login as this user").SetOptional()
	p.NewItem("dbPass", "string", "password for user").SetOptional()
	p.NewItem("dbHost", "string", "hostname of db")
	p.NewItem("dbPort", "int", "port of db").SetDefault(5432)
	p.NewItem("dbSslMode", "string",
		"one of: disable, require, verify-ca, verify-full").
		SetDefault("verify-full")
	p.NewItem("publicCert", "string", "file containing public cert").SetOptional()
	p.NewItem("privateKey", "string", "file containing private key").SetOptional()
	p.NewItem("caCerts", "string", "file containing CA certs").SetOptional()
	p.NewItem("dbConnectTimeout", "int", "seconds to attempt connect, 0 forever").
		SetDefault(0)
	p.NewItem("dbMaxConnLifetime", "duration",
		"max amount of time to reuse conns.  default is forever.").
		SetDefault("0s")
	p.NewItem("dbMaxIdleConns", "int", "max idle connects to db").
		SetDefault(2)
	p.NewItem("dbMaxOpenConns",
		"int", "max open connects to db.  Default unlimited.").
		SetDefault(0)
	p.NewItem("dbPrep", "[]string", "statements to run upon connect").SetOptional()
}

//
// uconfig builder for connecting to postgres
//
func BuildConnector(c *uconfig.Chain) (rv interface{}, err error) {

	g := &Connector{}
	g.SetDefaults()
	err = g.Construct(c)
	if nil == err {
		rv = g
	}
	return
}

//
// set defaults
//
func (this *Connector) SetDefaults() {
	this.SslMode = "verify-full"
	this.Port = 5432
}

//
// uconfig constructor for connecting to postgres
//
func (this *Connector) Construct(c *uconfig.Chain) (err error) {

	err = c.
		GetString("dbName", &this.Database, uconfig.StringNotBlank).
		GetString("dbUser", &this.User).
		GetString("dbPass", &this.Pass).
		GetString("dbHost", &this.Host, uconfig.StringNotBlank).
		GetPosInt("dbPort", &this.Port).
		GetString("dbSslMode", &this.SslMode, uconfig.StringNotBlank).
		GetString("publicCert", &this.PublicCertF).
		GetString("privateKey", &this.PrivateKeyF).
		GetString("caCerts", &this.CaCertsF).
		GetInt("dbConnectTimeout", &this.ConnectTimeout).
		GetInt("dbMaxOpenConns", &this.MaxOpenConns).
		GetInt("dbMaxIdleConns", &this.MaxIdleConns).
		GetDuration("dbMaxConnLifetime", &this.MaxConnLifetime).
		GetStrings("dbPrep", &this.Statements).
		Error
	if err != nil {
		return
	}
	switch this.SslMode {
	case "disable", "require":
		// empty
	case "verify-ca", "verify-full":
		if 0 == len(this.CaCertsF) {
			err = fmt.Errorf("SslMode is %s, but no caCerts provided", this.SslMode)
			return
		}
	default:
		err = fmt.Errorf("Invalid SslMode: '%s", this.SslMode)
		return
	}
	return
}
