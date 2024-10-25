package uconfig

import "testing"

func TestAddr(t *testing.T) {
	defaultHost := "host"
	defaultPort := "7"
	expectations := []struct {
		ok     bool
		addr   string
		expect string
	}{
		{true, "host:6", "host:6"},
		{true, "dashed-host:6", "dashed-host:6"},
		{true, "dotted.host:6", "dotted.host:6"},
		{true, "host", "host:7"},
		{true, "host:", "host:7"},
		{true, "1.2.3.4:", "1.2.3.4:7"},
		{true, ":", "host:7"},
		{true, "", "host:7"},
		{true, ":7", "host:7"},
		{true, "1.2.3.4:6", "1.2.3.4:6"},
		{true, "[::1]:6", "[::1]:6"},         // ipv6 requires []
		{true, "[::1]:", "[::1]:7"},          // ipv6 requires []
		{true, "::1", "[::1]:7"},             // but only if there is a port
		{true, "::", "[::]:7"},               // ditto
		{true, "[host]:6", "[host]:6"},       // [] works for other cases, too
		{true, "[1.2.3.4]:6", "[1.2.3.4]:6"}, //
		{true, "a.fully.qualified.name", "a.fully.qualified.name:7"},
		{true, "a-nother.fully.qualified.name", "a-nother.fully.qualified.name:7"},
		{false, "host:wow", "nope"},
		{false, "host:70000", "nope"},
		{false, "host:-3", "nope"},
		{false, "HOST:6", "nope"},       // only lower case
		{false, "camelCase:6", "nope"},  // only lower case
		{false, "snake_case:6", "nope"}, // underbars are not valid, but dash is
		{false, "-host:7", "nope"},      // no dash at begin or end
		{false, "host-:7", "nope"},
		{false, "-:7", "nope"},
		{false, "[::1:6]", "nope"}, // [] requird when port added
		{false, "[::1]", "nope"},   // if no port, then [] not allowed
	}

	for i, expect := range expectations {
		result, err := EnsureAddr(defaultHost, defaultPort, expect.addr)
		if expect.ok {
			if err != nil {
				t.Fatalf("%d got unexpected error: %s", i, err)
			} else if result != expect.expect {
				t.Fatalf("%d got %s, expected %s", i, result, expect.expect)
			}
		} else {
			if nil == err {
				t.Fatalf("%d did not get expected error", i)
			}
		}
	}
}
