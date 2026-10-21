// Command ocsfgen generates Go types for the OCSF classes and objects
// listed in gen.yaml from a compiled OCSF schema (the output of the OCSF
// schema compiler). See the README for the rules.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	schemaPath := flag.String("schema", "", "compiled OCSF schema (JSON)")
	configPath := flag.String("config", "gen.yaml", "allowlist of classes, objects and profiles")
	out := flag.String("out", "ocsf_gen.go", "generated Go file")
	pkg := flag.String("package", "ocsf", "package name of the generated file")
	flag.Parse()
	if err := run(*schemaPath, *configPath, *out, *pkg); err != nil {
		fmt.Fprintln(os.Stderr, "ocsfgen:", err)
		os.Exit(1)
	}
}

func run(schemaPath, configPath, out, pkg string) error {
	if schemaPath == "" {
		return fmt.Errorf("-schema is required")
	}
	s, err := os.ReadFile(schemaPath) //nolint:gosec // operator-supplied path
	if err != nil {
		return err
	}
	c, err := os.ReadFile(configPath) //nolint:gosec // operator-supplied path
	if err != nil {
		return err
	}
	cfg, err := parseConfig(c)
	if err != nil {
		return err
	}
	src, err := Generate(s, cfg, pkg)
	if err != nil {
		return err
	}
	return os.WriteFile(out, src, 0o644) //nolint:gosec // generated source is world-readable
}
