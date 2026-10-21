package ocsftest

import (
	"fmt"
	"testing"

	"github.com/luupback/ocsf"
)

type fakeTB struct {
	testing.TB
	errs []string
}

func (f *fakeTB) Helper() {}
func (f *fakeTB) Errorf(format string, args ...any) {
	f.errs = append(f.errs, fmt.Sprintf(format, args...))
}

func TestValid(t *testing.T) {
	ev := ocsf.Authentication{
		ClassUID: ocsf.AuthenticationClassUID, CategoryUID: ocsf.AuthenticationCategoryUID,
		ActivityID: ocsf.AuthenticationActivityIDLogoff, TypeUID: ocsf.AuthenticationClassUID*100 + ocsf.AuthenticationActivityIDLogoff,
		SeverityID: ocsf.AuthenticationSeverityIDInformational, Time: 1,
		Metadata: &ocsf.Metadata{Version: ocsf.SchemaVersion, Product: &ocsf.Product{Name: "test"}},
		User:     &ocsf.User{UID: "usr:1"}, Service: &ocsf.Service{Name: "texas"},
	}
	Valid(t, ev) // a struct
	f := &fakeTB{TB: t}
	ev.User = nil
	Valid(f, ev)
	if len(f.errs) == 0 {
		t.Error("an event without its required user passed")
	}
}
