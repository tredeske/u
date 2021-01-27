package golum

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
)

var (
	reloadableAdded_ bool
)

func setupReloadable() {
	if !reloadableAdded_ {
		reloadableAdded_ = true
		AddReloadable("reloadable", &reloadable_{})
	}
}

func TestReloadable(t *testing.T) {
	setupReloadable()

	err := TestLoadAndStart([]byte(`
components:
- name:         reloadable
  type:         reloadable
`))
	if err != nil {
		t.Fatalf("Unable to load and start: %s", err)
	}

	time.Sleep(50 * time.Millisecond)

}

func TestReloadableDelayedStart(t *testing.T) {
	setupReloadable()

	err := TestLoadAndStart([]byte(`
components:
- name:         delayed-r
  type:         reloadable
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

type reloadable_ struct {
	UnhelpfulReloadable
	Name  string
	delay time.Duration
}

// implement Reloadable
func (this *reloadable_) Reload(name string, c *uconfig.Section,
) (rv Reloadable, err error) {
	g := &reloadable_{Name: name}
	rv = g
	err = c.Chain().
		GetDuration("delay", &g.delay).
		Error
	return
}

// implement Startable
func (this *reloadable_) Start() (err error) {
	if 0 != this.delay {
		fmt.Printf("Delaying for %s", this.delay)
		time.Sleep(this.delay)
	}
	ulog.Printf("%s: started", this.Name)
	return
}

// implement Stopable
func (this *reloadable_) Stop() {
	ulog.Printf("%s: stopped", this.Name)
}
