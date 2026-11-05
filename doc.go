// Package ocsf has Go types for every OCSF class and object, generated from
// the compiled OCSF schema by cmd/ocsfgen (gen.yaml selects them; today
// all). It has no dependencies; validation lives in the validate and
// ocsftest packages so producers do not embed the schema, and the catalog
// package maps class and activity numbers to names.
package ocsf

//go:generate go run ./cmd/ocsfgen -schema validate/schema/ocsf-schema-v1.5.0.json -config gen.yaml -out ocsf_gen.go
