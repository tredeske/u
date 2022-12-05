package golum

import (
	"fmt"
	"io"
	"sort"

	"github.com/tredeske/u/uconfig"

	"gopkg.in/yaml.v2"
)

// show info about named component.  if name is 'all', then list all.
func Show(name string, out io.Writer) {

	if "all" == name {

		var names []string
		managers_.Range(func(k, v any) (cont bool) {
			names = append(names, k.(string))
			return true
		})
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintf(out, "%s\n", n)
		}

	} else {

		it, ok := managers_.Load(name)
		if !ok {
			fmt.Fprintf(out, "Unknown component: %s\n", name)
			return
		}
		//mgr := it.(Manager)
		mgr := it.(reloadableMgr_)

		help := &uconfig.Help{}
		mgr.HelpGolum(name, help)

		content, err := yaml.Marshal(help)
		if err != nil {
			fmt.Fprintf(out, "Error creating help for %s: %s", name, err)
			return
		}
		out.Write(content)
	}
}
