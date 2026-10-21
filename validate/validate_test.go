package validate_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/luupback/ocsf"
	"github.com/luupback/ocsf/validate"
)

func metadata() *ocsf.Metadata {
	return &ocsf.Metadata{Version: ocsf.SchemaVersion, Product: &ocsf.Product{Name: "test", VendorName: "luupback"}}
}

func logon() ocsf.Authentication {
	return ocsf.Authentication{
		ClassUID: ocsf.AuthenticationClassUID, CategoryUID: ocsf.AuthenticationCategoryUID,
		ActivityID: ocsf.AuthenticationActivityIDLogon, TypeUID: ocsf.AuthenticationClassUID*100 + ocsf.AuthenticationActivityIDLogon,
		SeverityID: ocsf.AuthenticationSeverityIDInformational, Time: 1_700_000_000_000, Metadata: metadata(),
		User: &ocsf.User{UID: "usr:1"}, Service: &ocsf.Service{Name: "texas"},
		Actor: &ocsf.Actor{User: &ocsf.User{UID: "svc:a/b"}},
	}
}

func check(t *testing.T, v any) validate.Result {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	r, err := validate.Check(b)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestEmbeddedSchemaVersion(t *testing.T) {
	var s struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(validate.Schema(), &s); err != nil {
		t.Fatal(err)
	}
	if s.Version != ocsf.SchemaVersion {
		t.Fatalf("embedded schema %s, generated types %s", s.Version, ocsf.SchemaVersion)
	}
}

func TestValidEvents(t *testing.T) {
	for name, ev := range map[string]any{
		"authentication": logon(),
		"account_change": ocsf.AccountChange{
			ClassUID: ocsf.AccountChangeClassUID, CategoryUID: ocsf.AccountChangeCategoryUID,
			ActivityID: ocsf.AccountChangeActivityIDCreate, TypeUID: ocsf.AccountChangeClassUID*100 + ocsf.AccountChangeActivityIDCreate,
			SeverityID: ocsf.AccountChangeSeverityIDInformational, Time: 1, Metadata: metadata(), User: &ocsf.User{UID: "usr:1"},
		},
		"detection_finding": ocsf.DetectionFinding{
			ClassUID: ocsf.DetectionFindingClassUID, CategoryUID: ocsf.DetectionFindingCategoryUID,
			ActivityID: ocsf.DetectionFindingActivityIDCreate, TypeUID: ocsf.DetectionFindingClassUID*100 + ocsf.DetectionFindingActivityIDCreate,
			SeverityID: ocsf.DetectionFindingSeverityIDMedium, Time: 1, Metadata: metadata(),
			FindingInfo: &ocsf.FindingInfo{UID: "f1", Title: "t"},
			Evidences:   []ocsf.Evidences{{SrcEndpoint: &ocsf.NetworkEndpoint{IP: "203.0.113.9"}}},
		},
		"entity_management": ocsf.EntityManagement{
			ClassUID: ocsf.EntityManagementClassUID, CategoryUID: ocsf.EntityManagementCategoryUID,
			ActivityID: ocsf.EntityManagementActivityIDUpdate, TypeUID: ocsf.EntityManagementClassUID*100 + ocsf.EntityManagementActivityIDUpdate,
			SeverityID: ocsf.EntityManagementSeverityIDInformational, Time: 1, Metadata: metadata(),
			Entity: &ocsf.ManagedEntity{Name: "routes"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if r := check(t, ev); len(r.Findings) > 0 {
				t.Errorf("findings: %v", r.Findings)
			}
		})
	}
}

func TestInvalidEvents(t *testing.T) {
	noUser := logon()
	noUser.User = nil
	noService := logon()
	noService.Service = nil
	badEnum := logon()
	badEnum.ActivityID = 42
	cases := map[string]struct {
		event any
		want  string
	}{
		"missing required user":       {noUser, `"user"`},
		"at_least_one service/dst":    {noService, "at_least_one"},
		"activity_id not in its enum": {badEnum, "activity_id"},
		"unknown attribute": {map[string]any{
			"class_uid": 3002, "category_uid": 3, "activity_id": 1, "type_uid": 300201, "severity_id": 1, "time": 1,
			"metadata": metadata(), "user": map[string]any{"uid": "u"}, "service": map[string]any{"name": "s"}, "bogus": 1,
		}, "bogus"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b, _ := json.Marshal(tc.event)
			err := validate.Event(b)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Event = %v, want an error mentioning %s", err, tc.want)
			}
		})
	}
	if _, err := validate.Check([]byte(`[1]`)); err == nil {
		t.Error("a JSON array was accepted as an event")
	}
}
