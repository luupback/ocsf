# luupback/ocsf

Go types for every [OCSF](https://schema.ocsf.io/) class and object, generated from the compiled OCSF schema; a catalogue mapping class and activity numbers to names; and validation of OCSF events with the official [ocsf-toolkit](https://github.com/ocsf/ocsf-toolkit).

Pinned schema: **OCSF 1.5.0** (`ocsf.SchemaVersion`).

| Package | What | Dependencies |
| --- | --- | --- |
| `github.com/luupback/ocsf` | Generated structs, enum constants (`AuthenticationActivityIDLogon`, `UserTypeIDUnknown`, …) and class constants (`AuthenticationClassUID`, `AuthenticationCategoryUID`). | none |
| `github.com/luupback/ocsf/catalog` | Every category, class and activity with its captions, generated as tables: `catalog.ClassByUID(3002)`, `catalog.ClassByName("authentication")`, `class.Activity(1)` (Logon), `class.TypeUID(1)`, `catalog.Classes()`, `catalog.Categories()`. Nothing is parsed at run time. | none |
| `github.com/luupback/ocsf/validate` | `validate.Event(json) error` and `validate.Check(json) (Result, error)`, checking against the embedded compiled schema with ocsf-toolkit. It honours `metadata.profiles`. | ocsf-toolkit, plus the embedded schema (~3 MB) |
| `github.com/luupback/ocsf/ocsftest` | `ocsftest.Valid(t, event)`: fails the test on any error or warning finding. | `validate` |

Producers import the types in production code and use `validate`/`ocsftest` only in tests, so their binaries carry neither the schema nor the toolkit. A service that validates at run time (a central audit service, say) can call `validate.Check` on every event: it is safe for concurrent use, and an Authentication logon takes about 15–19 µs, 5.2 KB and 82 allocations (`BenchmarkCheck`; about 4 µs per event across 8 goroutines, `BenchmarkCheckParallel`; Apple M1 Pro, Go 1.27). The schema is loaded once, on the first call.

```go
ev := ocsf.Authentication{
	ClassUID: ocsf.AuthenticationClassUID, CategoryUID: ocsf.AuthenticationCategoryUID,
	ActivityID: ocsf.AuthenticationActivityIDLogon,
	TypeUID:    ocsf.AuthenticationClassUID*100 + ocsf.AuthenticationActivityIDLogon,
	SeverityID: ocsf.AuthenticationSeverityIDInformational,
	Time:       time.Now().UnixMilli(),
	Metadata:   &ocsf.Metadata{Version: ocsf.SchemaVersion, Product: &ocsf.Product{Name: "texas", VendorName: "luupback"}},
	User:       &ocsf.User{UID: "usr:…"},
	Service:    &ocsf.Service{Name: "texas"},
}

// in a test
ocsftest.Valid(t, ev)

// names for numbers
c, _ := catalog.ClassByUID(ev.ClassUID)  // Authentication, category 3 "Identity & Access Management"
a, _ := c.Activity(ev.ActivityID)         // Logon
```

## What is generated

`gen.yaml` selects the classes, objects and profiles to generate; `classes` and `objects` are each a list of OCSF names or `all`. Today it is `classes: all` and `objects: all` (all 81 classes of OCSF 1.5.0, including `base_event` and the `win/` extension classes, and all 161 objects), with no profiles. `cmd/ocsfgen` writes `ocsf_gen.go` (about 930 KB) from `validate/schema/ocsf-schema-v<version>.json`, and with `-catalog` writes `catalog/catalog_gen.go` (about 33 KB) from the same schema.

- **Classes and objects:** a class or object struct has every attribute the compiled schema gives it, inherited ones included. Deprecated attributes are left out, and so are attributes that come only from a profile not listed in `gen.yaml`. A deprecated class or object is still generated, with a `Deprecated:` note.
- **Names:** OCSF snake_case becomes Go CamelCase, with initialisms upper-cased (`http_request` is `HTTPRequest`). An extension's prefix is kept as a word (`win/registry_key_activity` is `WinRegistryKeyActivity`, `win/reg_key` is `WinRegKey`). Enum constants are type + field + caption (`AuthenticationActivityIDLogon`); a type_uid caption drops the class caption (`AuthenticationTypeUIDLogon`).
- **Collisions** are resolved deterministically. Identifiers are reserved in a fixed order: type names (classes, then objects, each sorted by OCSF name), then each class's `ClassUID`/`CategoryUID` constants, then enum constants in output order. A colliding type name, class constant or struct field is a generator error (none occur in 1.5.0). An enum constant whose name is already taken, by another entity or by an earlier value of the same enum, gets its numeric value appended; if that is taken too, it is an error. No 1.5.0 enum constant needs the suffix.
- **Field types:**

  | OCSF | Go |
  | --- | --- |
  | `string_t` and its subtypes (`ip_t`, `uuid_t`, `url_t`, …), `datetime_t` | `string` |
  | `integer_t` | `int` |
  | `long_t`, `timestamp_t` | `int64` |
  | `float_t` | `float64` |
  | `boolean_t` | `bool` |
  | `json_t` | `any` |
  | a listed object | `*T`, or `[]T` for an array |
  | the generic `object` (e.g. `unmapped`) | `map[string]any` |

- **Unlisted objects** (only with an explicit `objects` list): an attribute whose object isn't listed is left out, and the struct's doc comment names every omission. To use one, add its object to `gen.yaml` and run `make generate`. A *required* attribute whose object isn't listed is a generator error.
- **JSON tags** are the OCSF attribute names, with `omitempty` on everything except required scalars. An optional enum set to `0` (Unknown) is therefore left out, which OCSF reads the same way.
- **Validation is the source of truth.** The types catch misspelled or misplaced fields at compile time, but they don't encode required attributes, `at_least_one` constraints or enum ranges. Those are what `validate` checks, so run `ocsftest.Valid` over every event a service builds.

## The catalogue

`catalog` is generated from the same schema as the types (`catalog/catalog_gen.go`, by `go generate`; the API is in `catalog/catalog.go`). It has every class, including `base_event` (class 0, in category 0 "other", captioned "Uncategorized" by the schema, which `Categories()` includes), and each class's `activity_id` values sorted by ID, `0 Unknown` and `99 Other` included. Class names are the schema's (`authentication`, `win/registry_key_activity`). Returned values are copies.

## Regenerating, bumping the schema or changing gen.yaml

```sh
make schema VERSION=1.6.0   # compile the OCSF release (git + python3; pinned ocsf-schema-compiler)
# point the //go:embed (validate/validate.go) and //go:generate (doc.go, catalog/catalog.go) paths at the new file
make generate check test    # regenerate, then test
```

CI runs `make lint lint-ci check test`. `make check` fails if `ocsf_gen.go` or `catalog/catalog_gen.go` is stale. `make generate` alone regenerates both.

## License

Apache-2.0. The OCSF schema (embedded under `validate/schema`, and the source of the generated descriptions) and ocsf-toolkit are both Apache-2.0, © the OCSF project contributors.
