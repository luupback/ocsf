// Package ocsftest asserts in tests that events are valid OCSF.
package ocsftest

import (
	"encoding/json"
	"testing"

	"github.com/luupback/ocsf/validate"
)

// Valid fails t unless event (JSON, or a value marshalled to JSON) is
// valid OCSF with no error or warning findings.
func Valid(t testing.TB, event any) {
	t.Helper()
	b, ok := event.([]byte)
	if !ok {
		var err error
		if b, err = json.Marshal(event); err != nil {
			t.Fatalf("ocsftest: marshal %T: %v", event, err)
		}
	}
	r, err := validate.Check(b)
	if err != nil {
		t.Fatalf("ocsftest: %v", err)
	}
	for _, f := range r.Findings {
		t.Errorf("ocsftest: %s\n  event: %s", f, b)
	}
}
