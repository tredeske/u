package u

import (
	"testing"

	"github.com/tredeske/u/golum"
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/uregistry"
)

//
// A thing to manage using golum
//
var theMgr_ thingMgr_

type thingMgr_ struct{}

type thing_ struct {
	name    string
	a       string
	b       string
	i       int64
	j       int64
	mystuff []stuff_
}
type stuff_ struct {
	foo string
	bar bool
}

func (this thingMgr_) NewGolum(name string, config *uconfig.Section) (err error) {

	g := &thing_{
		name: name,
		i:    -1,
		j:    50,
	}

	err = config.Chain().
		GetString("a", &g.a, uconfig.StringNotBlank).
		GetString("b", &g.b).
		GetInt("i", &g.i).
		GetInt("g.j", &g.j).

		//
		EachSection("stuff",
			func(s *uconfig.Section) (err error) {
				aStuff := stuff_{}
				return s.Chain().
					GetString("foo", &aStuff.foo).
					GetBool("bar", &aStuff.bar).
					Then(func() { g.mystuff = append(g.mystuff, aStuff) }).
					Error
			}).

		//
		Then(func() { uregistry.MustPutSingleton(name, g) }).
		Error
	return
}

func (this thingMgr_) StartGolum(name string) (err error) { return }

func (this thingMgr_) StopGolum(name string) { uregistry.Remove(name) }

func (this thingMgr_) ReloadGolum(name string, c *uconfig.Section) (err error) {
	this.StopGolum(name)
	err = this.NewGolum(name, c)
	if nil == err {
		err = this.StartGolum(name)
	}
	return
}

//
// put together a YAML config
//
var config_ = `
debug:
    enable: ["all"]

components:

- name:                   thingOne
  type:                   testFactory
  config:
    a:                    test
    b:                    "{{ .subValue }}"
    i:                    100
    stuff:
    - foo:                foo1
      bar:                true
    - foo:                800

- name:                   thingTwo
  type:                   testFactory
  config:
    a:                    test2
    i:                    101
    j:                    3
    stuff:
    - foo:                foo2
      bar:                false
    - foo:                false

substitutions:
  subValue:               a value to sub in
`

//
// A way to use golum in a test
//
func TestTesting(t *testing.T) {

	//
	// install the factory(ies)
	//
	golum.AddManager("testFactory", theMgr_)

	ulog.DebugEnabled = true

	//
	// start up your stuff from YAML config
	//
	err := golum.TestLoadAndStart(config_)
	defer golum.TestStop()
	if err != nil {
		t.Fatalf("Unable to config: %s", err)
	}

	//
	// do what you need to do
	//
	var thingOne, thingTwo *thing_
	err = uregistry.GetValid("thingOne", &thingOne)
	if err != nil {
		t.Fatalf("Unable to get thingOne: %s", err)
	}

	err = uregistry.GetValid("thingTwo", &thingTwo)
	if err != nil {
		t.Fatalf("Unable to get thingTwo: %s", err)
	}

}
