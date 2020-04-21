Utility library for golang
==========================

The utility library provides basic ingredients to create a standalone
program that is configured via YAML and which interacts sensibly with
its environment.
* dynamically update with configuration changes
* take appropriate action on signal
* log output sensibly

uboot
-----

This package can be used to bootstrap your program.  Here is a sample main.go:

    //
    // register lifecycle managers
    //
    func init() {
        golum.AddManager("thing", thing.TheMgr)
        golum.AddManager("foobar", foobar.TheMgr)
        ...
    }
    
    func main() {
    
        boot, err := uboot.SimpleBoot()
        if err != nil {
            ulog.Fatalf("Exitting due to startup failure: %s", err)
        }

        // ....
    
        uexit.SimpleSignalHandling() // park here until time to die
    }

In the YAML config, if 'reload' is set to true, uboot will detect
configuration changes and make them available to golum.

golum
-----

This package is used to create components configured via uconfig.  If the
YAML contains a 'components' array, then each configuration will connote
to a component to manage.  Here is an example:

    autoreload: true # uboot will detect config changes
    debug: false # turn off debugging

    components:
    - name: thingOne
      type: thingFactory
      config: { ... }
    - name: thingTwo
      type: thingFactory
      config: { ... }

uconfig
-------

This package is used to read and validate configuration settings.  It has a
fluent api as well as a standard api.

Each component loaded by golum will get a uconfig.Section that is used by
the registered factory to produce the component.

uerr
----

A simple error chaining package.

uexec
-----

For shelling out to run commands.

uexit
-----

For managing program exit and signals.

uinit
-----

Some initialization functions.

uio
---

Some handy doodads for i/o.

ulog
----

For logging output.

uregistry
---------

For registering things that need to be found in the program.

urest
-----

For interacting with RESTful services.

ustrings
--------

Some handy string and string slice thingees.

usync
-----

Some handy synchronization doodads, such as semaphores.


build & test
------------

    go get gopkg.in/yaml.v2
    go build
    go test . .u* ./golum
