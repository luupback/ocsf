// Package catalog maps OCSF numbers to names: every category, class and
// activity of the pinned OCSF schema, with their captions. Its tables are
// generated from the compiled schema by cmd/ocsfgen (catalog_gen.go), so it
// has no dependencies and parses nothing at run time.
//
// The returned values are copies; changing one does not change the
// catalogue.
package catalog

//go:generate go run ../cmd/ocsfgen -catalog -schema ../validate/schema/ocsf-schema-v1.5.0.json -package catalog -out catalog_gen.go

import "slices"

// Category is an OCSF event category. The base event's category (UID 0,
// "other", "Uncategorized") is included, though the schema does not list
// it among its categories.
type Category struct {
	UID     int
	Name    string // e.g. "iam"
	Caption string // e.g. "Identity & Access Management"
}

// Activity is one activity_id value of a class.
type Activity struct {
	ID      int    // activity_id
	Caption string // e.g. "Logon"
}

// Class is an OCSF event class.
type Class struct {
	UID        int    // class_uid, e.g. 3002
	Name       string // the schema name, e.g. "authentication" or "win/registry_key_activity"
	Caption    string // e.g. "Authentication"
	Category   Category
	Activities []Activity // sorted by ID; includes 0 Unknown and 99 Other where the schema has them
	Deprecated bool       // the schema deprecates the class
}

var (
	classByUID     = map[int]int{}    // class_uid -> index in classes
	classByName    = map[string]int{} // name -> index in classes
	categoryByUID  = map[int]int{}
	categoryByName = map[string]int{} // name -> index in categories
)

func init() {
	for i, c := range classes {
		classByUID[c.UID] = i
		classByName[c.Name] = i
	}
	for i, c := range categories {
		categoryByUID[c.UID] = i
		categoryByName[c.Name] = i
	}
}

func (c Class) clone() Class {
	c.Activities = slices.Clone(c.Activities)
	return c
}

// ClassByUID returns the class with this class_uid.
func ClassByUID(uid int) (Class, bool) {
	i, ok := classByUID[uid]
	if !ok {
		return Class{}, false
	}
	return classes[i].clone(), true
}

// ClassByName returns the class by its schema name (e.g. "authentication";
// extension classes keep their prefix, e.g. "win/registry_key_activity").
func ClassByName(name string) (Class, bool) {
	i, ok := classByName[name]
	if !ok {
		return Class{}, false
	}
	return classes[i].clone(), true
}

// Activity returns the activity_id within the class.
func (c Class) Activity(id int) (Activity, bool) {
	for _, a := range c.Activities {
		if a.ID == id {
			return a, true
		}
	}
	return Activity{}, false
}

// TypeUID is class_uid*100 + activity_id.
func (c Class) TypeUID(activityID int) int64 {
	return int64(c.UID)*100 + int64(activityID)
}

// Classes returns every class, sorted by UID.
func Classes() []Class {
	out := make([]Class, len(classes))
	for i, c := range classes {
		out[i] = c.clone()
	}
	return out
}

// Categories returns every category, sorted by UID.
func Categories() []Category {
	return slices.Clone(categories)
}

// CategoryByUID returns the category with this category_uid.
func CategoryByUID(uid int) (Category, bool) {
	i, ok := categoryByUID[uid]
	if !ok {
		return Category{}, false
	}
	return categories[i], true
}

// CategoryByName returns the category by its schema name (e.g. "iam").
func CategoryByName(name string) (Category, bool) {
	i, ok := categoryByName[name]
	if !ok {
		return Category{}, false
	}
	return categories[i], true
}
