package golum

import (
	"fmt"
	"io"
	"sort"

	"github.com/tredeske/u/uconfig"
)

// show info about named component type.  if kind is 'all', then list all.
func Show(kind string, out io.Writer) {

	if "all" == kind {

		fmt.Fprintf(out, `
Structure
=========

The YAML config file structure has 2 main sections.  The properties provides
key/value pairs that can be substituted throughout the rest of the config.

properties:
  key:      value            # associate key to value
  key2:     7                # associate key2 to numeric value
  key3:     "${ENV_VAR}"     # pull in an environment variable
  key4:     "foo-{{.key2}}"  # use value of key2 as part of this value

components:                  # a list of components to enable
- name:     NAME             # unique name of component
  type:     TYPE             # component type (see below)
  disabled: false            # (opt) is component disabled?
  timeout:  2s               # (opt) how long to wait for component to start
  hosts:    []               # (opt) hosts this component is valid for
  note:     words about this # (opt) a note
  config:                    # configuration for this component (see below)
    foo:    bar              # a simple config setting
    blah:   "{{.key}}"       # substitution from properties
    ...
- name:     NAME2            # 2nd component
    ...


The following properties are automatically added:
- homeDir        - the home dir of the user
- thisUser       - the username of the user
- thisHost       - the hostname (nodename) of the host
- thisIp         - the first listed (non loopback) IP
- thisProcess    - the process name of the process
- thisDir        - where the process executable is installed
- initDir        - where the process is started from

Other files can be included with the 'include_' directive, as in:

include_:        /path/to/file.yml


Available Components
====================

These are the available components.  To see the component specific configuration,
use the -show [component] command line parameter.

`)
		var kinds []string
		prototypes_.Range(func(k, v any) (cont bool) {
			kinds = append(kinds, k.(string))
			return true
		})
		sort.Strings(kinds)
		for _, n := range kinds {
			fmt.Fprintf(out, "\t%s\n", n)
		}

	} else {

		it, ok := prototypes_.Load(kind)
		if !ok {
			fmt.Fprintf(out, "Unknown component type: %s\n", kind)
			return
		}
		prototype := it.(Reloadable)

		help := &uconfig.Help{}
		prototype.Help(kind, help)

		content, err := help.AsYaml()
		if err != nil {
			fmt.Fprintf(out, "Error creating help for %s: %s", kind, err)
			return
		}
		out.Write(content)
	}
}
