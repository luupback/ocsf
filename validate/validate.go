// Package validate checks OCSF events against the compiled OCSF schema
// embedded in this package, with the official ocsf-toolkit validator. It is
// meant for producers' tests and CI (see ocsftest); importing it embeds the
// full compiled schema (about 3 MB) in the binary.
package validate

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/ocsf/ocsf-toolkit/eventpipeline"
	"github.com/ocsf/ocsf-toolkit/eventresult"
	"github.com/ocsf/ocsf-toolkit/jsonio"
	"github.com/ocsf/ocsf-toolkit/validation"

	"github.com/luupback/ocsf"
)

// schemaJSON is the compiled OCSF schema the types were generated from
// (scripts/compile-schema.sh).
//
//go:embed schema/ocsf-schema-v1.5.0.json
var schemaJSON []byte

// Schema returns the embedded compiled schema.
func Schema() []byte { return schemaJSON }

// Finding is one validation finding: a schema condition the event breaks.
type Finding struct {
	Warning bool   // false: an error
	Code    string // the toolkit's validation code name
	Message string // never quotes event values
}

func (f Finding) String() string {
	level := "error"
	if f.Warning {
		level = "warning"
	}
	return level + " " + f.Code + ": " + f.Message
}

// Result is every finding for one event.
type Result struct{ Findings []Finding }

// Err joins the error findings, or is nil when there are none.
func (r Result) Err() error {
	var errs []error
	for _, f := range r.Findings {
		if !f.Warning {
			errs = append(errs, errors.New(f.String()))
		}
	}
	return errors.Join(errs...)
}

var (
	once     sync.Once
	pipeline *eventpipeline.Pipeline
	initErr  error
)

func load() (*eventpipeline.Pipeline, error) {
	once.Do(func() {
		schema, issues, err := eventpipeline.NewSchemaFromBytes(schemaJSON)
		if err != nil {
			initErr = fmt.Errorf("ocsf: load schema %s: %w", ocsf.SchemaVersion, err)
			return
		}
		var msgs []string
		for _, i := range issues {
			msgs = append(msgs, fmt.Sprint(i.Code, ": ", i.Message))
		}
		if len(msgs) > 0 {
			initErr = fmt.Errorf("ocsf: schema %s initialization issues: %s", ocsf.SchemaVersion, strings.Join(msgs, "; "))
			return
		}
		pipeline, initErr = eventpipeline.NewPipeline(eventpipeline.WithSchema(schema), eventpipeline.WithValidation())
	})
	return pipeline, initErr
}

// Check validates one JSON event (honouring its metadata.profiles) and
// returns every finding. The error is for unusable input (not a JSON
// object) or an unusable schema, not for findings.
func Check(event []byte) (Result, error) {
	p, err := load()
	if err != nil {
		return Result{}, err
	}
	m, err := jsonio.DecodeObject(bytes.NewReader(event))
	if err != nil {
		return Result{}, fmt.Errorf("ocsf: decode event: %w", err)
	}
	res, err := p.ProcessEvent(m)
	if err != nil {
		return Result{}, fmt.Errorf("ocsf: process event: %w", err)
	}
	var out Result
	for _, f := range res.Validation().Findings {
		out.Findings = append(out.Findings, finding(f))
	}
	for _, i := range res.Issues() {
		out.Findings = append(out.Findings, Finding{Code: fmt.Sprint(i.Code), Message: i.Message})
	}
	return out, nil
}

func finding(f eventresult.ValidationFinding) Finding {
	return Finding{Warning: f.Level == validation.LevelWarning, Code: fmt.Sprint(f.Code), Message: f.Message}
}

// Event validates one JSON event and returns its error findings joined, or
// nil when it is valid OCSF.
func Event(event []byte) error {
	r, err := Check(event)
	if err != nil {
		return err
	}
	return r.Err()
}
