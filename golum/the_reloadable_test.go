package golum

import (
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/uregistry"
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

	log.Printf(`
GIVEN No components
 WHEN Load a test component
 THEN It loads AND is reachable
`)

	err := TestLoadAndStart([]byte(`
components:
- name:         reloadable
  type:         reloadable
`))
	if err != nil {
		t.Fatalf("Unable to load and start: %s", err)
	}

	time.Sleep(50 * time.Millisecond)

	var r *reloadable_
	uregistry.MustGet("reloadable", &r)

	log.Printf(`
GIVEN A component running
 WHEN Reload config
 THEN It reloads AND is reachable
`)

	err = TestReload([]byte(`
components:
- name:         reloadable
  type:         reloadable
  config:
    foo:        bar
`))

	uregistry.MustGet("reloadable", &r)
	if r.foo != "bar" {
		t.Fatalf("foo did not get updated to bar")
	}

	log.Printf(`
GIVEN Reloaded component running
 WHEN Reload config again
 THEN It reloads AND is reachable
`)

	err = TestReload([]byte(`
components:
- name:         reloadable
  type:         reloadable
  config:
    foo:        finally
`))

	uregistry.MustGet("reloadable", &r)
	if r.foo != "finally" {
		t.Fatalf("foo did not get updated to finally")
	}
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
	foo   string
}

// implement Reloadable
func (this *reloadable_) Reload(name string, c *uconfig.Chain,
) (rv Reloadable, err error) {
	g := &reloadable_{Name: name}
	rv = g
	err = c.
		GetDuration("delay", &g.delay).
		GetString("foo", &g.foo).
		Done()
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
