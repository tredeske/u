package golum

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/uregistry"
)

var (
	testAdded_ bool
)

func TestAuto(t *testing.T) {
	if !testAdded_ {
		AddManager("auto", &autoMgr_{})
		testAdded_ = true
	}

	err := TestLoadAndStart([]byte(`
components:
- name:         auto
  type:         auto
`))
	if err != nil {
		t.Fatalf("Unable to load and start: %s", err)
	}

	time.Sleep(50 * time.Millisecond)

}

func TestDelayedStart(t *testing.T) {
	if !testAdded_ {
		AddManager("auto", &autoMgr_{})
		testAdded_ = true
	}

	err := TestLoadAndStart([]byte(`
components:
- name:         delayed
  type:         auto
  config:
    delay:      15s
`))

	//
	// test passes if we get an error and error contains "timed out"
	//
	if err != nil {
		if !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("Unable to load and start: %s", err)
		}
	} else {
		t.Fatalf("Should have errored out due to delay")
	}
}

type autoMgr_ struct {
	AutoStartable
	AutoStoppable
	Unreloadable
}

func (this *autoMgr_) NewGolum(name string, c *uconfig.Section) (err error) {
	g := &auto_{Name: name}
	err = c.Chain().
		GetDuration("delay", &g.delay).
		Error
	if err != nil {
		return
	}
	uregistry.Put(name, g)
	return
}

type auto_ struct {
	Name  string
	delay time.Duration
}

// implement Startable
func (this *auto_) Start() (err error) {
	if 0 != this.delay {
		fmt.Printf("Delaying for %s", this.delay)
		time.Sleep(this.delay)
	}
	ulog.Printf("%s: started", this.Name)
	return
}

// implement Stopable
func (this *auto_) Stop() {
	ulog.Printf("%s: stopped", this.Name)
}
