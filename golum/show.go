package golum

import (
	"fmt"
	"io"
	"sort"

	"github.com/tredeske/u/uconfig"

	"gopkg.in/yaml.v2"
)

//
// implement this in your Manager if you provide helpful info
//
type Helper interface {
	HelpGolum(name string, help *uconfig.Help)
}

//
// show info about named component.  if name is 'all', then list all.
//
func Show(name string, out io.Writer) {

	if "all" == name {

		var names []string
		for k, _ := range managers_ {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintf(out, "%s\n", n)
		}

	} else {

		mgr := managers_[name]
		if nil == mgr {
			fmt.Fprintf(out, "Unknown component: %s\n", name)
			return
		}
		h, casted := mgr.(Helper)
		if !casted {
			fmt.Fprintf(out, "No help spec registered for component %s\n", name)
			return
		}

		help := &uconfig.Help{}
		h.HelpGolum(name, help)

		content, err := yaml.Marshal(help)
		if err != nil {
			fmt.Fprintf(out, "Error creating help for %s: %s", name, err)
			return
		}
		out.Write(content)
	}
}
