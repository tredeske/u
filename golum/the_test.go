package golum

import (
	"testing"
	"time"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/uregistry"
)

func TestAuto(t *testing.T) {
	AddManager("auto", &autoMgr_{})

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

type autoMgr_ struct {
	AutoStartable
	AutoStoppable
	Unreloadable
}

func (this *autoMgr_) NewGolum(name string, c *uconfig.Section) (err error) {
	g := &auto_{Name: name}
	uregistry.Put(name, g)
	return
}

type auto_ struct {
	Name string
}

// implement Startable
func (this *auto_) Start() (err error) {
	ulog.Printf("%s: started", this.Name)
	return
}

// implement Stopable
func (this *auto_) Stop() {
	ulog.Printf("%s: stopped", this.Name)
}
