// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package templatecheck

// Candidate types.
//
// When the data passed to a template is loosely typed, as with map[string]any,
// the type of a value such as .ppack is not known, and ordinary checking stops
// there: .ppack.Titel is accepted. With candidate types, the checker is given
// a set of types the values may have (the "universe"). A field access on a
// value of unknown type is then accepted only if some type in the universe has
// that field or method, and the possible result types are carried along the
// chain, so .ppack.Title.Foo is reported when no type that has a Title field
// has a Title with a Foo.
//
// This is a minimal check. A set of candidate types is used only to check that
// fields and methods exist. For everything else (comparisons, function
// arguments, printing) it behaves like an unknown type, so it never reports an
// error that ordinary checking would not.

import (
	htmpl "html/template"
	"reflect"
	ttmpl "text/template"
)

// CheckHTMLWithCandidates is like CheckHTML, but checks field accesses on
// values of unknown type against the types of candidates and the types
// reachable from them through exported fields and method results.
//
// A FieldNames value in candidates adds field names that are accepted without
// a type, for types that cannot be named (such as struct types declared inside
// a function, or anonymous struct types). A field access with one of these
// names gives a value of unknown type.
func CheckHTMLWithCandidates(t *htmpl.Template, typeValue any, candidates []any) error {
	return check(htmlTemplate{t}, typeValue, false, newUniverse(candidates))
}

// CheckTextWithCandidates is like CheckText, but checks field accesses on
// values of unknown type against candidates. See CheckHTMLWithCandidates.
func CheckTextWithCandidates(t *ttmpl.Template, typeValue any, candidates []any) error {
	return check(textTemplate{t}, typeValue, false, newUniverse(candidates))
}

// FieldNames is a list of field names to accept on values of unknown type.
// See CheckHTMLWithCandidates.
type FieldNames []string

// universe is the set of types a value of unknown type may have.
type universe struct {
	// names of fields accepted without a type (see FieldNames).
	names map[string]bool
	// types that may be the type of a value of unknown type. Pointers are
	// removed. Maps are excluded, because a map with string keys would accept
	// any field name and the check would never fail.
	types []reflect.Type
}

func newUniverse(samples []any) *universe {
	u := &universe{}
	seen := map[reflect.Type]bool{}
	var add func(t reflect.Type)
	add = func(t reflect.Type) {
		if t == nil {
			return
		}
		t = indirectType(t)
		if seen[t] {
			return
		}
		seen[t] = true
		switch t.Kind() {
		case reflect.Slice, reflect.Array, reflect.Chan:
			add(t.Elem())
			return
		case reflect.Map:
			add(t.Key())
			add(t.Elem())
			return
		case reflect.Struct:
			for i := 0; i < t.NumField(); i++ {
				if f := t.Field(i); f.PkgPath == "" || f.Anonymous {
					add(f.Type)
				}
			}
		}
		u.types = append(u.types, t)
		ptr := t
		if ptr.Kind() != reflect.Interface {
			ptr = reflect.PtrTo(ptr)
		}
		for i := 0; i < ptr.NumMethod(); i++ {
			if mt := ptr.Method(i).Type; mt.NumOut() > 0 {
				add(mt.Out(0))
			}
		}
	}
	for _, s := range samples {
		if names, ok := s.(FieldNames); ok {
			if u.names == nil {
				u.names = map[string]bool{}
			}
			for _, n := range names {
				u.names[n] = true
			}
			continue
		}
		add(reflect.TypeOf(s))
	}
	return u
}

// candidateMarker is the element type of the array types that stand for
// candidate sets. The set for type [n]candidateMarker is state.candSets[n].
type candidateMarker struct{}

var candidateMarkerType = reflect.TypeOf(candidateMarker{})

func isCandidateSet(t reflect.Type) bool {
	return t != nil && t.Kind() == reflect.Array && t.Elem() == candidateMarkerType
}

// isUnknown reports whether t is the unknown type or a set of candidate types,
// which outside field accesses behaves like the unknown type.
func isUnknown(t reflect.Type) bool {
	return t == unknownType || isCandidateSet(t)
}

// candidateSetType returns a type standing for the set of types.
func (s *state) candidateSetType(types []reflect.Type) reflect.Type {
	s.candSets = append(s.candSets, types)
	return reflect.ArrayOf(len(s.candSets)-1, candidateMarkerType)
}

// usesCandidates reports whether a field access on receiver should be checked
// against candidate types.
func (s *state) usesCandidates(receiver reflect.Type) bool {
	if s.universe == nil || s.strict || receiver == nil {
		return false
	}
	if isUnknown(receiver) {
		return true
	}
	r := indirectType(receiver)
	return r.Kind() == reflect.Interface && r.NumMethod() == 0
}

// evalCandidateField checks that some candidate type of receiver has the field
// or method, and returns the set of its possible types.
func (s *state) evalCandidateField(fieldName string, receiver reflect.Type) reflect.Type {
	from := s.universe.types
	if isCandidateSet(receiver) {
		from = s.candSets[receiver.Len()]
	}
	matched, anything := false, false
	seen := map[reflect.Type]bool{}
	var next []reflect.Type
	for _, t := range from {
		rt, ok := memberType(t, fieldName)
		if !ok {
			continue
		}
		matched = true
		if rt == nil {
			continue
		}
		rt = indirectType(rt)
		if isUnknown(rt) || (rt.Kind() == reflect.Interface && rt.NumMethod() == 0) {
			// One of the candidates gives a value of unknown type, so the
			// result may be anything.
			anything = true
			continue
		}
		if !seen[rt] {
			seen[rt] = true
			next = append(next, rt)
		}
	}
	if !matched {
		if s.universe.names[fieldName] {
			// A field of a type that cannot be named; we don't know its type.
			return unknownType
		}
		s.errorKindf(ErrNoCandidateType, fieldName, "no candidate type has field or method %q", fieldName)
	}
	if anything {
		return unknownType
	}
	return s.candidateSetType(next)
}

// memberType returns the type of the field or method named name of a value of
// type t, as the template engine would see it. For a method, it is the type of
// the first result, or nil if the method has none.
func memberType(t reflect.Type, name string) (reflect.Type, bool) {
	ptr := t
	if ptr.Kind() != reflect.Interface && ptr.Kind() != reflect.Ptr {
		ptr = reflect.PtrTo(ptr)
	}
	if m, ok := ptr.MethodByName(name); ok {
		if m.Type.NumOut() == 0 {
			return nil, true
		}
		return m.Type.Out(0), true
	}
	switch t.Kind() {
	case reflect.Struct:
		if f, ok := t.FieldByName(name); ok && f.PkgPath == "" {
			return f.Type, true
		}
	case reflect.Map:
		// A map with string keys accepts any field name as a key.
		if stringType.AssignableTo(t.Key()) {
			return t.Elem(), true
		}
	}
	return nil, false
}

// candidateElemType returns the type of the elements when ranging over a value
// whose type is one of the candidates in the set typ.
func (s *state) candidateElemType(typ reflect.Type) reflect.Type {
	seen := map[reflect.Type]bool{}
	var elems []reflect.Type
	for _, t := range s.candSets[typ.Len()] {
		var e reflect.Type
		switch t.Kind() {
		case reflect.Array, reflect.Slice, reflect.Chan, reflect.Map:
			e = indirectType(t.Elem())
		default:
			// Not something we know how to range over; be conservative.
			return unknownType
		}
		if isUnknown(e) || (e.Kind() == reflect.Interface && e.NumMethod() == 0) {
			return unknownType
		}
		if !seen[e] {
			seen[e] = true
			elems = append(elems, e)
		}
	}
	if len(elems) == 0 {
		return unknownType
	}
	return s.candidateSetType(elems)
}

// kindOf is t.Kind(), or reflect.Invalid for a nil t.
func kindOf(t reflect.Type) reflect.Kind {
	if t == nil {
		return reflect.Invalid
	}
	return t.Kind()
}
