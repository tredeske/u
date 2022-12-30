package golum

import (
	"fmt"
	"io"
	"sort"

	"github.com/tredeske/u/uconfig"

	"gopkg.in/yaml.v2"
)

// show info about named component type.  if kind is 'all', then list all.
func Show(kind string, out io.Writer) {

	if "all" == kind {

		fmt.Fprintf(out, `
components:
- name:     NAME             # unique name of component
  type:     TYPE             # see list, below
  disabled: false            # is component disabled?
  timeout:  2s               # how long to wait for component to start
  hosts:    []               # hosts this component is valid for
  note:     a few words about this
  config:
    foo:    bar              # component specific configuration
    ...

`)
		var kinds []string
		prototypes_.Range(func(k, v any) (cont bool) {
			kinds = append(kinds, k.(string))
			return true
		})
		sort.Strings(kinds)
		for _, n := range kinds {
			fmt.Fprintf(out, "%s\n", n)
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

		content, err := yaml.Marshal(help)
		if err != nil {
			fmt.Fprintf(out, "Error creating help for %s: %s", kind, err)
			return
		}
		out.Write(content)
	}
}
