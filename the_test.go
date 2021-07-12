package u

import (
	"testing"
	"time"

	"github.com/tredeske/u/golum"
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/uregistry"
)

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

func (this *thing_) Help(name string, help *uconfig.Help) {
	p := help.Init(name, "This thing is a test")
	p.NewItem("a", "string", "test element a")
	p.NewItem("b", "string", "test element b")
	p.NewItem("i", "int", "test element i")
	p.NewItem("j", "int", "test element j")
}

func (this *thing_) Reload(
	name string,
	config *uconfig.Section,
) (
	rv golum.Reloadable,
	err error,
) {

	g := &thing_{
		name: name,
		i:    -1,
		j:    50,
	}
	rv = g

	err = config.Chain().
		GetString("a", &g.a, uconfig.StringNotBlank()).
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

func (this *thing_) Start() (err error) { return }

func (this *thing_) Stop() {}

//
// put together a YAML config
//
var config_ = `
debug:
  enable:                 ["all"]

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

properties:
  subValue:               a value to sub in
`

//
// A way to use golum in a test
//
func TestTesting(t *testing.T) {

	//
	// install the factory(ies)
	//
	golum.AddReloadable("testFactory", &thing_{})

	//
	// start up your stuff from YAML config
	//
	err := golum.TestLoadAndStart(config_)
	defer golum.TestStop()
	if err != nil {
		t.Fatalf("Unable to config: %s", err)
	}

	time.Sleep(time.Second)
	if !ulog.DebugEnabled {
		t.Fatalf("debug not enabled for all")
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
