// Command ocsfgen generates Go code from a compiled OCSF schema (the output
// of the OCSF schema compiler): the types for the classes and objects
// listed in gen.yaml, or, with -catalog, the catalog package's tables of
// every category, class and activity. See the README for the rules.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	schemaPath := flag.String("schema", "", "compiled OCSF schema (JSON)")
	configPath := flag.String("config", "gen.yaml", "allowlist of classes, objects and profiles (not used with -catalog)")
	catalog := flag.Bool("catalog", false, "generate the catalog package's tables instead of types")
	out := flag.String("out", "ocsf_gen.go", "generated Go file")
	pkg := flag.String("package", "ocsf", "package name of the generated file")
	flag.Parse()
	if err := run(*schemaPath, *configPath, *catalog, *out, *pkg); err != nil {
		fmt.Fprintln(os.Stderr, "ocsfgen:", err)
		os.Exit(1)
	}
}

func run(schemaPath, configPath string, catalog bool, out, pkg string) error {
	if schemaPath == "" {
		return errors.New("-schema is required")
	}
	s, err := os.ReadFile(schemaPath) //nolint:gosec // operator-supplied path
	if err != nil {
		return err
	}
	var src []byte
	if catalog {
		src, err = GenerateCatalog(s, pkg)
	} else {
		var c []byte
		c, err = os.ReadFile(configPath) //nolint:gosec // operator-supplied path
		if err != nil {
			return err
		}
		var cfg Config
		if cfg, err = parseConfig(c); err != nil {
			return err
		}
		src, err = Generate(s, cfg, pkg)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(out, src, 0o644) //nolint:gosec // generated source is world-readable
}
