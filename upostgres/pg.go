package upostgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/tredeske/u/uerr"
)

//
// enable connecting to postgres db server
//
type Connector struct {
	Database        string
	User            string
	Pass            string
	Host            string
	Port            int
	SslMode         string
	PublicCertF     string
	PrivateKeyF     string
	CaCertsF        string
	ConnectTimeout  int
	MaxOpenConns    int
	MaxIdleConns    int
	MaxConnLifetime time.Duration
	Statements      []string
}

//
// manufacture connection string
//
func (this *Connector) String() (rv string) {
	b := strings.Builder{}
	b.WriteString("dbname=")
	b.WriteString(this.Database)
	this.addString(&b, "user", this.User)
	this.addString(&b, "password", this.Pass)
	this.addString(&b, "host", this.Host)
	this.addString(&b, "sslmode", this.SslMode)
	this.addString(&b, "sslcert", this.PublicCertF)
	this.addString(&b, "sslkey", this.PrivateKeyF)
	this.addString(&b, "sslrootcert", this.CaCertsF)
	if 0 != this.ConnectTimeout {
		this.addString(&b, "connect_timeout", strconv.Itoa(this.ConnectTimeout))
	}
	if 0 != this.Port {
		this.addString(&b, "port", strconv.Itoa(this.Port))
	}
	return b.String()
}
func (this *Connector) addString(b *strings.Builder, key, value string) {
	if 0 != len(value) {
		b.WriteRune(' ')
		b.WriteString(key)
		b.WriteRune('=')
		b.WriteString(value)
	}
}

//
// connect to the specified db server
//
func (this *Connector) Connect() (rv *sql.DB, err error) {

	if this.Database != strings.ToLower(this.Database) {
		err = fmt.Errorf("Database name (%s) must be all lower case", this.Database)
		return
	}

	connectS := this.String()
	rv, err = sql.Open("postgres", connectS)
	if err != nil {
		err = uerr.Chainf(err, "Connecting to postgres with '%s'", connectS)
		return
	}

	rv.SetMaxIdleConns(this.MaxIdleConns)
	rv.SetMaxOpenConns(this.MaxOpenConns)
	rv.SetConnMaxLifetime(this.MaxConnLifetime)

	//
	// perform on connect prep statements, if any
	//
	var statement *sql.Stmt
	for _, s := range this.Statements {

		statement, err = rv.Prepare(s)
		if err != nil {
			return
		}

		_, err = statement.Exec()
		statement.Close()
		if err != nil {
			return
		}
	}
	return
}

//
// create a prepared statement
//
func Prepare(db *sql.DB, statement string) (rv *sql.Stmt, err error) {
	rv, err = db.Prepare(statement)
	if err != nil {
		err = uerr.Chainf(err, "Unable to prepare '%s'", statement)
	}
	return
}

//
// Perform a transaction over multiple statements
//
func Transact(
	db *sql.DB,
	action func(*sql.Tx) error,
	opts *sql.TxOptions,
) (
	err error,
) {

	var tx *sql.Tx

	defer func() {
		if nil != tx {
			tx.Rollback()
		}
	}()

	tx, err = db.BeginTx(context.Background(), opts)
	if err != nil {
		return
	}

	err = action(tx)
	if err != nil {
		return
	}

	err = tx.Commit()
	tx = nil // prevent rollback

	return
}

/*
//
// Perform a write statement
//
func Write(
	db *sql.DB,
	statement string,
	action func(*sql.Stmt) error,
) (err error) {

	var stmt *sql.Stmt

	defer func() {
		if nil != stmt {
			stmt.Close()
		}
	}()

	stmt, err = db.Prepare(statement)
	if err != nil {
		err = uerr.Chainf(err, "Preparing '%s'", statement)
		return
	}

	err = action(stmt)
	if err != nil {
		return
	}
	return
}
*/
