package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"html"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Config is gen.yaml: what to generate.
type Config struct {
	Classes  names    `yaml:"classes"`
	Objects  names    `yaml:"objects"`
	Profiles []string `yaml:"profiles"`
}

// names is a list of OCSF names, or every name: the YAML scalar "all" (or
// "*") is stored as the single element all.
type names []string

const all = "*"

// UnmarshalYAML accepts a sequence of names or the scalar "all" / "*".
func (n *names) UnmarshalYAML(v *yaml.Node) error {
	if v.Kind == yaml.ScalarNode {
		if v.Value != "all" && v.Value != all {
			return fmt.Errorf("line %d: want a list or \"all\", got %q", v.Line, v.Value)
		}
		*n = names{all}
		return nil
	}
	var l []string
	if err := v.Decode(&l); err != nil {
		return err
	}
	*n = l
	return nil
}

// resolve expands all into every name in the schema and checks that listed
// names exist. The result is sorted.
func (n names) resolve(kind, version string, in map[string]entity) ([]string, error) {
	if slices.Contains(n, all) {
		return slices.Sorted(maps.Keys(in)), nil
	}
	for _, x := range n {
		if _, ok := in[x]; !ok {
			return nil, fmt.Errorf("%s %q is not in OCSF %s", kind, x, version)
		}
	}
	out := slices.Clone([]string(n))
	slices.Sort(out)
	return slices.Compact(out), nil
}

func parseConfig(b []byte) (Config, error) {
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("config: %w", err)
	}
	if len(c.Classes) == 0 {
		return c, errors.New("config: no classes")
	}
	return c, nil
}

// The compiled schema, as far as the generator reads it.
type schema struct {
	Version    string            `json:"version"`
	Classes    map[string]entity `json:"classes"`
	Objects    map[string]entity `json:"objects"`
	Categories struct {
		Attributes map[string]struct {
			UID     int    `json:"uid"`
			Caption string `json:"caption"`
		} `json:"attributes"`
	} `json:"categories"`
	Dictionary struct {
		Types struct {
			Attributes map[string]struct {
				Type string `json:"type"`
			} `json:"attributes"`
		} `json:"types"`
	} `json:"dictionary"`
}

type entity struct {
	Name        string              `json:"name"`
	Caption     string              `json:"caption"`
	Description string              `json:"description"`
	UID         int                 `json:"uid"`
	Category    string              `json:"category"`
	CategoryUID int                 `json:"category_uid"`
	Attributes  map[string]attr     `json:"attributes"`
	Constraints map[string][]string `json:"constraints"`
	Deprecated  *deprecation        `json:"@deprecated"`
}

type deprecation struct {
	Message string `json:"message"`
	Since   string `json:"since"`
}

type attr struct {
	Caption     string               `json:"caption"`
	Description string               `json:"description"`
	Type        string               `json:"type"`
	ObjectType  string               `json:"object_type"`
	IsArray     bool                 `json:"is_array"`
	Requirement string               `json:"requirement"`
	Enum        map[string]enumValue `json:"enum"`
	Profiles    []string             `json:"profiles"`
	Deprecated  json.RawMessage      `json:"@deprecated"`
}

type enumValue struct {
	Caption string `json:"caption"`
}

// genericObject is OCSF's untyped object (unmapped, for example).
const genericObject = "object"

func parseSchema(schemaJSON []byte) (schema, error) {
	var s schema
	if err := json.Unmarshal(schemaJSON, &s); err != nil {
		return s, fmt.Errorf("schema: %w", err)
	}
	if s.Version == "" {
		return s, errors.New("schema: no version")
	}
	return s, nil
}

// Generate renders the Go source for cfg from the compiled schema.
//
// Go identifiers are reserved in a fixed order, so the output never depends
// on map order: first every type name (classes, then objects, each sorted
// by OCSF name), then every class's ClassUID and CategoryUID constants,
// then enum constants entity by entity in output order (classes, then
// objects, attributes sorted). A type name, class constant or struct field
// that collides is an error (none do in OCSF 1.5.0). An enum constant that
// collides with an identifier already reserved (by another entity, or by
// an earlier value of its own enum) gets its numeric value appended, and is
// an error if that collides too.
func Generate(schemaJSON []byte, cfg Config, pkg string) ([]byte, error) {
	s, err := parseSchema(schemaJSON)
	if err != nil {
		return nil, err
	}
	classes, err := cfg.Classes.resolve("class", s.Version, s.Classes)
	if err != nil {
		return nil, err
	}
	objects, err := cfg.Objects.resolve("object", s.Version, s.Objects)
	if err != nil {
		return nil, err
	}
	g := &gen{s: s, objects: map[string]bool{}, profiles: map[string]bool{}, names: map[string]string{"SchemaVersion": "SchemaVersion"}}
	for _, o := range objects {
		g.objects[o] = true
	}
	for _, p := range cfg.Profiles {
		g.profiles[p] = true
	}
	for _, c := range classes {
		if err := g.claim(goName(c), "class "+c); err != nil {
			return nil, err
		}
	}
	for _, o := range objects {
		if err := g.claim(goName(o), "object "+o); err != nil {
			return nil, err
		}
	}
	for _, c := range classes {
		for _, suffix := range []string{"ClassUID", "CategoryUID"} {
			if err := g.claim(goName(c)+suffix, "class "+c); err != nil {
				return nil, err
			}
		}
	}

	fmt.Fprintf(&g.b, "// Code generated by cmd/ocsfgen from OCSF %s; DO NOT EDIT.\n\n", s.Version)
	fmt.Fprintf(&g.b, "package %s\n\n", pkg)
	fmt.Fprintf(&g.b, "// SchemaVersion is the OCSF schema version these types were generated from.\nconst SchemaVersion = %q\n\n", s.Version)

	for _, c := range classes {
		e := s.Classes[c]
		name := goName(c)
		fmt.Fprintf(&g.b, "// %s class constants.\nconst (\n\t%sClassUID = %d\n\t%sCategoryUID = %d\n)\n\n", name, name, e.UID, name, e.CategoryUID)
		if err := g.entity(c, e, true); err != nil {
			return nil, err
		}
	}
	for _, o := range objects {
		if err := g.entity(o, s.Objects[o], false); err != nil {
			return nil, err
		}
	}
	src, err := format.Source(g.b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}
	return src, nil
}

type gen struct {
	s        schema
	objects  map[string]bool
	profiles map[string]bool
	names    map[string]string // Go identifier -> what claimed it
	b        bytes.Buffer
}

// claim reserves a top-level Go identifier.
func (g *gen) claim(ident, owner string) error {
	if prev, ok := g.names[ident]; ok {
		return fmt.Errorf("%s and %s both map to Go identifier %s", owner, prev, ident)
	}
	g.names[ident] = owner
	return nil
}

func (g *gen) entity(name string, e entity, isClass bool) error {
	typ := goName(name)
	kind := "object"
	if isClass {
		kind = "class"
	}
	fmt.Fprintf(&g.b, "// %s is the OCSF %s %s %q.", typ, g.s.Version, kind, e.Caption)
	if d := firstSentence(e.Description); d != "" {
		fmt.Fprintf(&g.b, " %s", d)
	}
	g.b.WriteString("\n")
	if len(e.Constraints) > 0 {
		for _, k := range slices.Sorted(maps.Keys(e.Constraints)) {
			fmt.Fprintf(&g.b, "//\n// Constraint %s: %s.\n", k, strings.Join(e.Constraints[k], ", "))
		}
	}
	var omitted []string
	var fields bytes.Buffer
	var enums bytes.Buffer
	fieldNames := map[string]string{}
	for _, an := range slices.Sorted(maps.Keys(e.Attributes)) {
		a := e.Attributes[an]
		if len(a.Deprecated) > 0 && string(a.Deprecated) != "null" {
			continue
		}
		if !g.inProfiles(a.Profiles) {
			continue
		}
		goType, ok, err := g.goType(a)
		if err != nil {
			return fmt.Errorf("%s %s.%s: %w", kind, name, an, err)
		}
		if !ok {
			if a.Requirement == "required" {
				return fmt.Errorf("%s %s: required attribute %s needs object %q in gen.yaml", kind, name, an, a.ObjectType)
			}
			omitted = append(omitted, an+" ("+a.ObjectType+")")
			continue
		}
		field := goName(an)
		if prev, ok := fieldNames[field]; ok {
			return fmt.Errorf("%s %s: attributes %s and %s both map to Go field %s", kind, name, prev, an, field)
		}
		fieldNames[field] = an
		tag := an
		scalar := !strings.HasPrefix(goType, "*") && !strings.HasPrefix(goType, "[]") && !strings.HasPrefix(goType, "map")
		if a.Requirement != "required" || !scalar {
			tag += ",omitempty"
		}
		fmt.Fprintf(&fields, "\t// %s (%s)", a.Caption, a.Requirement)
		if d := firstSentence(a.Description); d != "" {
			fmt.Fprintf(&fields, ": %s", d)
		}
		fmt.Fprintf(&fields, "\n\t%s %s `json:%q`\n", field, goType, tag)
		if len(a.Enum) > 0 && !a.IsArray {
			if err := g.writeEnum(&enums, typ+field, kind+" "+name+"."+an, e.Caption, a.Enum); err != nil {
				return err
			}
		}
	}
	if len(omitted) > 0 {
		g.b.WriteString("//\n// Not generated (object not in gen.yaml): ")
		g.b.WriteString(strings.Join(omitted, ", "))
		g.b.WriteString(".\n")
	}
	if e.Deprecated != nil {
		fmt.Fprintf(&g.b, "//\n// Deprecated: since OCSF %s. %s\n", e.Deprecated.Since, plain(e.Deprecated.Message))
	}
	fmt.Fprintf(&g.b, "type %s struct {\n%s}\n\n", typ, fields.Bytes())
	g.b.Write(enums.Bytes())
	return nil
}

func (g *gen) inProfiles(ps []string) bool {
	if len(ps) == 0 {
		return true
	}
	for _, p := range ps {
		if g.profiles[p] {
			return true
		}
	}
	return false
}

// goType maps an OCSF attribute type; ok is false for an object that is
// not allowlisted.
func (g *gen) goType(a attr) (t string, ok bool, err error) {
	if a.Type == "object_t" {
		switch {
		case a.ObjectType == genericObject:
			t = "map[string]any"
		case g.objects[a.ObjectType]:
			t = "*" + goName(a.ObjectType)
			if a.IsArray {
				t = goName(a.ObjectType)
			}
		default:
			return "", false, nil
		}
	} else {
		base := a.Type
		if d, ok := g.s.Dictionary.Types.Attributes[base]; ok && d.Type != "" {
			base = d.Type
		}
		switch base {
		case "string_t", "datetime_t":
			t = "string"
		case "integer_t":
			t = "int"
		case "long_t", "timestamp_t":
			t = "int64"
		case "float_t":
			t = "float64"
		case "boolean_t":
			t = "bool"
		case "json_t":
			t = "any"
		default:
			return "", false, fmt.Errorf("unknown type %q", a.Type)
		}
	}
	if a.IsArray {
		t = "[]" + t
	}
	return t, true, nil
}

type enumEntry struct {
	v int
	n string
}

// intEnum is an integer enum sorted by value; ok is false for a string
// enum.
func intEnum(enum map[string]enumValue) (vals []enumEntry, ok bool) {
	for k, e := range enum {
		v, err := strconv.Atoi(k)
		if err != nil {
			return nil, false
		}
		vals = append(vals, enumEntry{v, e.Caption})
	}
	slices.SortFunc(vals, func(a, b enumEntry) int { return a.v - b.v })
	return vals, true
}

// writeEnum writes the constants of an integer enum. A caption that
// repeats the entity's ("Authentication: Logon", in type_uid) is named
// without it. See Generate for how colliding names are resolved.
func (g *gen) writeEnum(b *bytes.Buffer, prefix, owner, entityCaption string, enum map[string]enumValue) error {
	vals, ok := intEnum(enum)
	if !ok {
		return nil // a string enum; no constants
	}
	fmt.Fprintf(b, "// %s values.\nconst (\n", prefix)
	for _, e := range vals {
		n := prefix + identifier(strings.TrimPrefix(e.n, entityCaption+": "))
		if _, taken := g.names[n]; taken {
			n += strconv.Itoa(e.v)
		}
		if err := g.claim(n, owner+" = "+strconv.Itoa(e.v)); err != nil {
			return err
		}
		fmt.Fprintf(b, "\t%s = %d // %s\n", n, e.v, e.n)
	}
	b.WriteString(")\n\n")
	return nil
}

// GenerateCatalog renders the catalog package's tables (every category,
// and every class with its activities) from the compiled schema.
func GenerateCatalog(schemaJSON []byte, pkg string) ([]byte, error) {
	s, err := parseSchema(schemaJSON)
	if err != nil {
		return nil, err
	}
	type cat struct {
		uid           int
		name, caption string
	}
	cats := map[string]cat{}
	for n, c := range s.Categories.Attributes {
		cats[n] = cat{c.UID, n, c.Caption}
	}
	byUID := map[int]string{}
	for _, n := range slices.Sorted(maps.Keys(s.Classes)) {
		c := s.Classes[n]
		if prev, ok := byUID[c.UID]; ok {
			return nil, fmt.Errorf("classes %s and %s share class_uid %d", prev, n, c.UID)
		}
		byUID[c.UID] = n
		k, ok := cats[c.Category]
		if !ok {
			// base_event's category ("other", 0) is not among the schema's
			// categories; its caption is the class's category_uid enum's.
			vals, _ := intEnum(c.Attributes["category_uid"].Enum)
			if c.Category == "" || len(vals) != 1 || vals[0].v != c.CategoryUID {
				return nil, fmt.Errorf("class %s: unknown category %q", n, c.Category)
			}
			k = cat{c.CategoryUID, c.Category, vals[0].n}
			cats[c.Category] = k
		}
		if k.uid != c.CategoryUID {
			return nil, fmt.Errorf("class %s: category %s has uid %d, class says %d", n, c.Category, k.uid, c.CategoryUID)
		}
	}
	catByUID := map[int]cat{}
	for _, k := range cats {
		if prev, ok := catByUID[k.uid]; ok {
			return nil, fmt.Errorf("categories %s and %s share uid %d", prev.name, k.name, k.uid)
		}
		catByUID[k.uid] = k
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by cmd/ocsfgen from OCSF %s; DO NOT EDIT.\n\n", s.Version)
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	fmt.Fprintf(&b, "// SchemaVersion is the OCSF schema version the catalogue was generated from.\nconst SchemaVersion = %q\n\n", s.Version)
	b.WriteString("// categories is sorted by UID.\nvar categories = []Category{\n")
	for _, uid := range slices.Sorted(maps.Keys(catByUID)) {
		k := catByUID[uid]
		fmt.Fprintf(&b, "\t{UID: %d, Name: %q, Caption: %q},\n", k.uid, k.name, k.caption)
	}
	b.WriteString("}\n\n")
	b.WriteString("// classes is sorted by UID, and each class's activities by ID.\nvar classes = []Class{\n")
	for _, uid := range slices.Sorted(maps.Keys(byUID)) {
		n := byUID[uid]
		c := s.Classes[n]
		k := cats[c.Category]
		acts, ok := intEnum(c.Attributes["activity_id"].Enum)
		if !ok || len(acts) == 0 {
			return nil, fmt.Errorf("class %s: no activity_id enum", n)
		}
		fmt.Fprintf(&b, "\t{\n\t\tUID: %d, Name: %q, Caption: %q,\n", c.UID, n, c.Caption)
		fmt.Fprintf(&b, "\t\tCategory: Category{UID: %d, Name: %q, Caption: %q},\n", k.uid, k.name, k.caption)
		if c.Deprecated != nil {
			b.WriteString("\t\tDeprecated: true,\n")
		}
		b.WriteString("\t\tActivities: []Activity{\n")
		for _, a := range acts {
			fmt.Fprintf(&b, "\t\t\t{ID: %d, Caption: %q},\n", a.v, a.n)
		}
		b.WriteString("\t\t},\n\t},\n")
	}
	b.WriteString("}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}
	return src, nil
}

// initialisms are upper-cased whole in Go names.
var initialisms = map[string]bool{
	"id": true, "uid": true, "ip": true, "url": true, "uri": true, "http": true, "https": true, "dns": true, "os": true,
	"mfa": true, "ttl": true, "api": true, "uuid": true, "tls": true, "ssl": true, "mac": true, "cpu": true, "vpn": true,
	"sid": true, "acl": true, "ldap": true, "sql": true, "html": true, "json": true, "tcp": true, "udp": true, "vm": true,
	"ssh": true, "smb": true, "rdp": true, "ja4": true, "kb": true, "cve": true, "cvss": true, "cwe": true, "ssid": true,
}

// goName turns an OCSF snake_case name into an exported Go identifier. An
// extension's prefix is kept as a word: win/reg_key is WinRegKey.
func goName(n string) string {
	var b strings.Builder
	for _, p := range strings.FieldsFunc(n, func(r rune) bool { return r == '_' || r == '/' }) {
		if initialisms[p] {
			b.WriteString(strings.ToUpper(p))
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)

// identifier turns an enum caption ("Service Ticket Request") into a Go
// identifier part ("ServiceTicketRequest").
func identifier(caption string) string {
	var b strings.Builder
	for _, w := range nonAlnum.Split(caption, -1) {
		if w == "" {
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	out := b.String()
	if out == "" || out[0] >= '0' && out[0] <= '9' {
		out = "V" + out
	}
	return out
}

var tags = regexp.MustCompile(`<[^>]*>`)

// plain is HTML description text as one line of plain text.
func plain(d string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tags.ReplaceAllString(d, ""))), " ")
}

// firstSentence is the description's first sentence as plain text.
func firstSentence(d string) string {
	d = plain(d)
	if i := strings.Index(d, ". "); i >= 0 {
		d = d[:i+1]
	}
	return d
}
