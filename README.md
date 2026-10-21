# luupback/ocsf

Go types for the [OCSF](https://schema.ocsf.io/) classes and objects luupback services emit, generated from the compiled OCSF schema, and validation of OCSF events with the official [ocsf-toolkit](https://github.com/ocsf/ocsf-toolkit).

Pinned schema: **OCSF 1.5.0** (`ocsf.SchemaVersion`).

| Package | What | Dependencies |
| --- | --- | --- |
| `github.com/luupback/ocsf` | Generated structs, enum constants (`AuthenticationActivityIDLogon`, `UserTypeIDUnknown`, …) and class constants (`AuthenticationClassUID`, `AuthenticationCategoryUID`). | none |
| `github.com/luupback/ocsf/validate` | `validate.Event(json) error` and `validate.Check(json) (Result, error)`, checking against the embedded compiled schema with ocsf-toolkit. It honours `metadata.profiles`. | ocsf-toolkit, plus the embedded schema (~3 MB) |
| `github.com/luupback/ocsf/ocsftest` | `ocsftest.Valid(t, event)`: fails the test on any error or warning finding. | `validate` |

Producers import the types in production code and use `validate`/`ocsftest` only in tests, so their binaries carry neither the schema nor the toolkit.

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
```

## What is generated

`gen.yaml` lists the classes, objects and profiles to generate. `cmd/ocsfgen` writes `ocsf_gen.go` from `validate/schema/ocsf-schema-v<version>.json`.

- **Classes and objects:** a class or object struct has every attribute the compiled schema gives it, inherited ones included. Deprecated attributes are left out, and so are attributes that come only from a profile not listed in `gen.yaml`.
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

- **Unlisted objects:** an attribute whose object isn't listed is left out, and the struct's doc comment names every omission. To use one, add its object to `gen.yaml` and run `make generate`. A *required* attribute whose object isn't listed is a generator error.
- **JSON tags** are the OCSF attribute names, with `omitempty` on everything except required scalars. An optional enum set to `0` (Unknown) is therefore left out, which OCSF reads the same way.
- **Validation is the source of truth.** The types catch misspelled or misplaced fields at compile time, but they don't encode required attributes, `at_least_one` constraints or enum ranges. Those are what `validate` checks, so run `ocsftest.Valid` over every event a service builds.

## Bumping the schema or the allowlist

```sh
make schema VERSION=1.6.0   # compile the OCSF release (git + python3; pinned ocsf-schema-compiler)
# point the //go:embed (validate/validate.go) and //go:generate (doc.go) paths at the new file
make generate check test    # regenerate, then test
```

CI runs `make lint lint-ci check test`. `make check` fails if `ocsf_gen.go` is stale.

## License

Apache-2.0. The OCSF schema (embedded under `validate/schema`, and the source of the generated descriptions) and ocsf-toolkit are both Apache-2.0, © the OCSF project contributors.
