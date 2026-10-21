// Package ocsf has Go types for the OCSF classes and objects luupback
// services emit, generated from the compiled OCSF schema by cmd/ocsfgen
// (gen.yaml lists them). It has no dependencies; validation lives in the
// validate and ocsftest packages so producers do not embed the schema.
package ocsf

//go:generate go run ./cmd/ocsfgen -schema validate/schema/ocsf-schema-v1.5.0.json -config gen.yaml -out ocsf_gen.go
