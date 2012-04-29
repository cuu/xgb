package main

import (
	"encoding/xml"
	"io/ioutil"
	"log"
	"time"
)

type XML struct {
	// Root 'xcb' element properties.
	XMLName xml.Name `xml:"xcb"`
	Header string `xml:"header,attr"`
	ExtensionXName string `xml:"extension-xname,attr"`
	ExtensionName string `xml:"extension-name,attr"`
	MajorVersion string `xml:"major-version,attr"`
	MinorVersion string `xml:"minor-version,attr"`

	// Types for all top-level elements.
	// First are the simple ones.
	Imports Imports `xml:"import"`
	Enums Enums `xml:"enum"`
	Xids Xids `xml:"xidtype"`
	XidUnions Xids `xml:"xidunion"`
	TypeDefs TypeDefs `xml:"typedef"`
	EventCopies EventCopies `xml:"eventcopy"`
	ErrorCopies ErrorCopies `xml:"errorcopy"`

	// Here are the complex ones, i.e., anything with "structure contents"
	Structs Structs `xml:"struct"`
	Unions Unions `xml:"union"`
	Requests Requests `xml:"request"`
	Events Events `xml:"event"`
	Errors Errors `xml:"error"`
}

// Morph cascades down all of the XML and calls each type's corresponding
// Morph function with itself as an argument (the context).
func (x *XML) Morph(c *Context) {
	// Start the header...
	c.Putln("package xgb")
	c.Putln("/*")
	c.Putln("\tX protocol API for '%s.xml'.", c.xml.Header)
	c.Putln("\tThis file is automatically generated. Edit at your own peril!")
	c.Putln("\tGenerated on %s",
		time.Now().Format("Jan 2, 2006 at 3:04:05pm MST"))
	c.Putln("*/")
	c.Putln("")

	x.Imports.Morph(c)
	c.Putln("")

	x.Enums.Morph(c)
	c.Putln("")

	x.Xids.Morph(c)
	c.Putln("")

	x.XidUnions.Morph(c)
	c.Putln("")

	x.TypeDefs.Morph(c)
	c.Putln("")

	x.Structs.Morph(c)
	c.Putln("")

	x.Unions.Morph(c)
	c.Putln("")

	x.Requests.Morph(c)
	c.Putln("")

	x.Errors.Morph(c)
	c.Putln("")

	x.ErrorCopies.Morph(c)
	c.Putln("")

	x.Events.Morph(c)
	c.Putln("")

	x.EventCopies.Morph(c)
	c.Putln("")
}

// IsResource returns true if the 'needle' type is a resource type.
// i.e., an "xid"
func (x *XML) IsResource(needle Type) bool {
	for _, xid := range x.Xids {
		if needle == xid.Name {
			return true
		}
	}
	for _, xidunion := range x.XidUnions {
		if needle == xidunion.Name {
			return true
		}
	}
	for _, imp := range x.Imports {
		if imp.xml.IsResource(needle) {
			return true
		}
	}
	return false
}

// HasType returns true if the 'needle' type can be found in the protocol
// description represented by 'x'.
func (x *XML) HasType(needle Type) bool {
	for _, enum := range x.Enums {
		if needle == enum.Name {
			return true
		}
	}
	for _, xid := range x.Xids {
		if needle == xid.Name {
			return true
		}
	}
	for _, xidunion := range x.XidUnions {
		if needle == xidunion.Name {
			return true
		}
	}
	for _, typedef := range x.TypeDefs {
		if needle == typedef.New {
			return true
		}
	}
	for _, evcopy := range x.EventCopies {
		if needle == evcopy.Name {
			return true
		}
	}
	for _, errcopy := range x.ErrorCopies {
		if needle == errcopy.Name {
			return true
		}
	}
	for _, strct := range x.Structs {
		if needle == strct.Name {
			return true
		}
	}
	for _, union := range x.Unions {
		if needle == union.Name {
			return true
		}
	}
	for _, ev := range x.Events {
		if needle == ev.Name {
			return true
		}
	}
	for _, err := range x.Errors {
		if needle == err.Name {
			return true
		}
	}

	return false
}

type Name string

type Type string

// Union returns the 'Union' struct corresponding to this type, if
// one exists.
func (typ Type) Union(c *Context) *Union {
	// If this is a typedef, use that instead.
	if oldTyp, ok := typ.TypeDef(c); ok {
		return oldTyp.Union(c)
	}

	// Otherwise, just look for a union type with 'typ' name.
	for _, union := range c.xml.Unions {
		if typ == union.Name {
			return union
		}
	}
	for _, imp := range c.xml.Imports {
		for _, union := range imp.xml.Unions {
			if typ == union.Name {
				return union
			}
		}
	}
	return nil
}

// TypeDef returns the 'old' type corresponding to this type, if it's found
// in a type def. If not found, the second return value is false.
func (typ Type) TypeDef(c *Context) (Type, bool) {
	for _, typedef := range c.xml.TypeDefs {
		if typ == typedef.New {
			return typedef.Old, true
		}
	}
	for _, imp := range c.xml.Imports {
		for _, typedef := range imp.xml.TypeDefs {
			if typ == typedef.New {
				return typedef.Old, true
			}
		}
	}
	return "", false
}

// Size is a nifty function that takes any type and digs until it finds
// its underlying base type. At which point, the size can be determined.
func (typ Type) Size(c *Context) uint {
	// If this is a base type, we're done.
	if size, ok := BaseTypeSizes[string(typ)]; ok {
		return size
	}

	// If it's a resource, we're also done.
	if c.xml.IsResource(typ) {
		return BaseTypeSizes["Id"]
	}

	// It's not, so that implies there is *some* typedef declaring it
	// in terms of another type. Just follow that chain until we get to
	// a base type. We also need to check imported stuff.
	for _, typedef := range c.xml.TypeDefs {
		if typ == typedef.New {
			return typedef.Old.Size(c)
		}
	}
	for _, imp := range c.xml.Imports {
		for _, typedef := range imp.xml.TypeDefs {
			if typ == typedef.New {
				return typedef.Old.Size(c)
			}
		}
	}
	log.Panicf("Could not find base size of type '%s'.", typ)
	panic("unreachable")
}

type Imports []*Import

func (imports Imports) Eval() {
	for _, imp := range imports {
		xmlBytes, err := ioutil.ReadFile(*protoPath + "/" + imp.Name + ".xml")
		if err != nil {
			log.Fatalf("Could not read X protocol description for import " +
				"'%s' because: %s", imp.Name, err)
		}

		imp.xml = &XML{}
		err = xml.Unmarshal(xmlBytes, imp.xml)
		if err != nil {
			log.Fatal("Could not parse X protocol description for import " +
				"'%s' because: %s", imp.Name, err)
		}
	}
}

type Import struct {
	Name string `xml:",chardata"`
	xml *XML `xml:"-"`
}

type Enums []Enum

// Eval on the list of all enum types goes through and forces every enum
// item to have a valid expression.
// This is necessary because when an item is empty, it is defined to have
// the value of "one more than that of the previous item, or 0 for the first
// item".
func (enums Enums) Eval() {
	for _, enum := range enums {
		nextValue := uint(0)
		for _, item := range enum.Items {
			if item.Expr == nil {
				item.Expr = newValueExpression(nextValue)
				nextValue++
			} else {
				nextValue = item.Expr.Eval() + 1
			}
		}
	}
}

type Enum struct {
	Name Type `xml:"name,attr"`
	Items []*EnumItem `xml:"item"`
}

type EnumItem struct {
	Name Name `xml:"name,attr"`
	Expr *Expression `xml:",any"`
}

type Xids []*Xid

type Xid struct {
	XMLName xml.Name
	Name Type `xml:"name,attr"`
}

type TypeDefs []*TypeDef

type TypeDef struct {
	Old Type `xml:"oldname,attr"`
	New Type `xml:"newname,attr"`
}

type EventCopies []*EventCopy

type EventCopy struct {
	Name Type `xml:"name,attr"`
	Number int `xml:"number,attr"`
	Ref Type `xml:"ref,attr"`
}

type ErrorCopies []*ErrorCopy

type ErrorCopy struct {
	Name Type `xml:"name,attr"`
	Number int `xml:"number,attr"`
	Ref Type `xml:"ref,attr"`
}

type Structs []*Struct

type Struct struct {
	Name Type `xml:"name,attr"`
	Fields Fields `xml:",any"`
}

type Unions []*Union

type Union struct {
	Name Type `xml:"name,attr"`
	Fields Fields `xml:",any"`
}

type Requests []*Request

type Request struct {
	Name Type `xml:"name,attr"`
	Opcode int `xml:"opcode,attr"`
	Combine bool `xml:"combine-adjacent,attr"`
	Fields Fields `xml:",any"`
	Reply *Reply `xml:"reply"`
}

type Reply struct {
	Fields Fields `xml:",any"`
}

type Events []*Event

type Event struct {
	Name Type `xml:"name,attr"`
	Number int `xml:"number,attr"`
	NoSequence bool `xml:"no-sequence-number,true"`
	Fields Fields `xml:",any"`
}

type Errors []*Error

type Error struct {
	Name Type `xml:"name,attr"`
	Number int `xml:"number,attr"`
	Fields Fields `xml:",any"`
}

