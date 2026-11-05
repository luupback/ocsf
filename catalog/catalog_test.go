package catalog_test

import (
	"encoding/json"
	"slices"
	"strconv"
	"testing"

	"github.com/luupback/ocsf"
	"github.com/luupback/ocsf/catalog"
	"github.com/luupback/ocsf/validate"
)

// The schema, as far as the tests compare it with the catalogue (test-only:
// the catalogue itself parses nothing).
type schema struct {
	Version string `json:"version"`
	Classes map[string]struct {
		UID         int    `json:"uid"`
		Caption     string `json:"caption"`
		Category    string `json:"category"`
		CategoryUID int    `json:"category_uid"`
		Attributes  map[string]struct {
			Enum map[string]struct {
				Caption string `json:"caption"`
			} `json:"enum"`
		} `json:"attributes"`
	} `json:"classes"`
	Categories struct {
		Attributes map[string]struct {
			UID     int    `json:"uid"`
			Caption string `json:"caption"`
		} `json:"attributes"`
	} `json:"categories"`
}

func load(t *testing.T) schema {
	t.Helper()
	var s schema
	if err := json.Unmarshal(validate.Schema(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEveryClassResolves(t *testing.T) {
	s := load(t)
	if s.Version != catalog.SchemaVersion || catalog.SchemaVersion != ocsf.SchemaVersion {
		t.Fatalf("schema %s, catalog %s, types %s", s.Version, catalog.SchemaVersion, ocsf.SchemaVersion)
	}
	if got := len(catalog.Classes()); got != len(s.Classes) {
		t.Errorf("%d classes in the catalogue, %d in the schema", got, len(s.Classes))
	}
	for name, sc := range s.Classes {
		c, ok := catalog.ClassByUID(sc.UID)
		if !ok {
			t.Errorf("class %s (%d) not found by UID", name, sc.UID)
			continue
		}
		if c.Name != name || c.Caption != sc.Caption || c.Category.UID != sc.CategoryUID || c.Category.Name != sc.Category {
			t.Errorf("class %d: got %+v", sc.UID, c)
		}
		if byName, ok := catalog.ClassByName(name); !ok || byName.UID != sc.UID {
			t.Errorf("class %s not found by name", name)
		}
		enum := sc.Attributes["activity_id"].Enum
		if len(c.Activities) != len(enum) {
			t.Errorf("class %s: %d activities, schema has %d", name, len(c.Activities), len(enum))
		}
		for k, v := range enum {
			id, err := strconv.Atoi(k)
			if err != nil {
				t.Fatal(err)
			}
			if a, ok := c.Activity(id); !ok || a.Caption != v.Caption {
				t.Errorf("class %s activity %d: got %+v %v, want %q", name, id, a, ok, v.Caption)
			}
		}
		if _, ok := enum["99"]; ok {
			if a, ok := c.Activity(99); !ok || a.Caption != "Other" {
				t.Errorf("class %s: activity 99 = %+v %v", name, a, ok)
			}
		}
		if !slices.IsSortedFunc(c.Activities, func(a, b catalog.Activity) int { return a.ID - b.ID }) {
			t.Errorf("class %s: activities not sorted", name)
		}
	}
	for name, sc := range s.Categories.Attributes {
		k, ok := catalog.CategoryByUID(sc.UID)
		if !ok || k.Name != name || k.Caption != sc.Caption {
			t.Errorf("category %s: got %+v %v", name, k, ok)
		}
	}
}

func TestAuthenticationLogon(t *testing.T) {
	c, ok := catalog.ClassByUID(3002)
	if !ok || c.Name != "authentication" || c.Caption != "Authentication" {
		t.Fatalf("3002: %+v %v", c, ok)
	}
	if c.Category != (catalog.Category{UID: 3, Name: "iam", Caption: "Identity & Access Management"}) {
		t.Errorf("category %+v", c.Category)
	}
	a, ok := c.Activity(1)
	if !ok || a != (catalog.Activity{ID: 1, Caption: "Logon"}) {
		t.Errorf("activity 1: %+v %v", a, ok)
	}
	if a, ok := c.Activity(0); !ok || a.Caption != "Unknown" {
		t.Errorf("activity 0: %+v %v", a, ok)
	}
	if a, ok := c.Activity(99); !ok || a.Caption != "Other" {
		t.Errorf("activity 99: %+v %v", a, ok)
	}
	if _, ok := c.Activity(42); ok {
		t.Error("activity 42 found")
	}
	if got := c.TypeUID(1); got != 300201 {
		t.Errorf("TypeUID(1) = %d", got)
	}
	if c.UID != ocsf.AuthenticationClassUID || c.Category.UID != ocsf.AuthenticationCategoryUID {
		t.Error("catalogue and generated constants disagree")
	}
	if byName, ok := catalog.ClassByName("authentication"); !ok || byName.UID != 3002 {
		t.Errorf("by name: %+v %v", byName, ok)
	}
}

func TestAPIActivity(t *testing.T) {
	c, ok := catalog.ClassByUID(6003)
	if !ok || c.Name != "api_activity" || c.Caption != "API Activity" || c.Category.Name != "application" {
		t.Fatalf("6003: %+v %v", c, ok)
	}
	if a, ok := c.Activity(2); !ok || a.Caption != "Read" {
		t.Errorf("activity 2: %+v %v", a, ok)
	}
}

func TestExtensionClass(t *testing.T) {
	c, ok := catalog.ClassByName("win/registry_key_activity")
	if !ok || c.UID != 201001 || c.Category.UID != 1 {
		t.Errorf("win/registry_key_activity: %+v %v", c, ok)
	}
}

func TestNotFound(t *testing.T) {
	if _, ok := catalog.ClassByUID(12345); ok {
		t.Error("class 12345 found")
	}
	if _, ok := catalog.ClassByName("nope"); ok {
		t.Error("class nope found")
	}
	if _, ok := catalog.CategoryByUID(42); ok {
		t.Error("category 42 found")
	}
}

func TestSorted(t *testing.T) {
	cs := catalog.Classes()
	if !slices.IsSortedFunc(cs, func(a, b catalog.Class) int { return a.UID - b.UID }) {
		t.Error("classes not sorted by UID")
	}
	ks := catalog.Categories()
	if !slices.IsSortedFunc(ks, func(a, b catalog.Category) int { return a.UID - b.UID }) {
		t.Error("categories not sorted by UID")
	}
	if len(ks) != 9 || ks[0].UID != 0 || ks[8].Name != "unmanned_systems" {
		t.Errorf("categories: %+v", ks)
	}
}

func TestCopies(t *testing.T) {
	c, _ := catalog.ClassByUID(3002)
	c.Activities[1].Caption = "changed"
	cs := catalog.Classes()
	cs[0].Name = "changed"
	if again, _ := catalog.ClassByUID(3002); again.Activities[1].Caption != "Logon" {
		t.Error("ClassByUID returned the catalogue's own activities")
	}
	if catalog.Classes()[0].Name == "changed" {
		t.Error("Classes returned the catalogue's own slice")
	}
}

func BenchmarkClassByUID(b *testing.B) {
	for b.Loop() {
		c, _ := catalog.ClassByUID(3002)
		_, _ = c.Activity(1)
	}
}
