package main

import "gopkg.in/yaml.v3"

// yamlUnmarshal is a thin wrapper so package-private callers can avoid the
// yaml.v3 import directly.
func yamlUnmarshal(raw string, out any) error {
	return yaml.Unmarshal([]byte(raw), out)
}
