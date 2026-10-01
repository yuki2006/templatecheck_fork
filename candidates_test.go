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

import (
	"errors"
	htmpl "html/template"
	"testing"
	ttmpl "text/template"
	"time"
)

type candUser struct {
	Name string
}

func (u *candUser) Display() string { return u.Name }

type candProblem struct {
	Title  string
	Author *candUser
	Tags   []string
	Date   time.Time
	Extra  map[string]int
	Any    any
}

type candWidget struct {
	Whatever string
}

type candContest struct {
	Name     string
	Problems []candProblem
}

func TestCheckHTMLWithCandidates(t *testing.T) {
	candidates := []any{candProblem{}, candContest{}}
	funcs := htmpl.FuncMap{"upper": func(s string) string { return s }}
	for _, test := range []struct {
		name    string
		tmpl    string
		subject string // "" means no error
	}{
		{"field", `{{.p.Title}}`, ""},
		{"nested field", `{{.p.Author.Name}}`, ""},
		{"method on pointer receiver", `{{.p.Author.Display}}`, ""},
		{"method of a reachable stdlib type", `{{.p.Date.Format "2006"}}`, ""},
		{"map with string keys accepts any key", `{{.p.Extra.anything}}`, ""},
		// A value of interface type is also assumed to be one of the candidates.
		{"field of a value of interface type", `{{.p.Any.Whatever}}`, "Whatever"},
		{"range over unknown", `{{range .ps}}{{.Title}}{{end}}`, ""},
		{"range over a known slice", `{{range .c.Problems}}{{.Author.Name}}{{end}}`, ""},
		{"with", `{{with .p}}{{.Author.Name}}{{end}}`, ""},
		{"variable", `{{$x := .p}}{{$x.Title}}`, ""},
		{"function result", `{{(upper .s)}}`, ""},
		{"ordered comparison of unknown values", `{{if gt .n 0}}x{{end}}`, ""},
		{"index of unknown value", `{{index .m "k"}}`, ""},

		{"misspelled field", `{{.p.Titel}}`, "Titel"},
		{"field of a field that does not have it", `{{.p.Title.Name}}`, "Name"},
		{"misspelled field in range over unknown", `{{range .ps}}{{.Nmae}}{{end}}`, "Nmae"},
		{"misspelled field in with", `{{with .p}}{{.Author.Nmae}}{{end}}`, "Nmae"},
		{"misspelled field of a variable", `{{$x := .p}}{{$x.Foo}}`, "Foo"},
	} {
		t.Run(test.name, func(t *testing.T) {
			tm := htmpl.Must(htmpl.New("t").Funcs(funcs).Parse(test.tmpl))
			err := CheckHTMLWithCandidates(tm, map[string]any{}, candidates)
			if test.name == "field of a value of interface type" {
				// Once a type with the field is a candidate, it is accepted.
				if err := CheckHTMLWithCandidates(tm, map[string]any{}, append(candidates, candWidget{})); err != nil {
					t.Fatalf("with candWidget: got %v, want no error", err)
				}
			}
			if test.subject == "" {
				if err != nil {
					t.Fatalf("got %v, want no error", err)
				}
				return
			}
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("got %v, want *Error", err)
			}
			if e.Kind != ErrNoCandidateType || e.Subject != test.subject {
				t.Errorf("got Kind=%d Subject=%q (%v), want ErrNoCandidateType %q", e.Kind, e.Subject, err, test.subject)
			}
		})
	}
}

// Without candidates, a field access on a value of unknown type is not checked,
// as before.
func TestCheckHTMLWithoutCandidatesUnchanged(t *testing.T) {
	tm := htmpl.Must(htmpl.New("t").Parse(`{{.p.Titel}}{{range .ps}}{{.Nmae}}{{end}}`))
	if err := CheckHTML(tm, map[string]any{}); err != nil {
		t.Fatalf("got %v, want no error", err)
	}
}

// Two candidate sets in one template do not get mixed up.
func TestCandidateSetsAreDistinct(t *testing.T) {
	tm := htmpl.Must(htmpl.New("t").Parse(`{{.p.Title}}{{.c.Name}}{{.p.Author.Name}}`))
	if err := CheckHTMLWithCandidates(tm, map[string]any{}, []any{candProblem{}, candContest{}}); err != nil {
		t.Fatal(err)
	}
}

// The types of the entries of the data map are used for their keys.
func TestDataMapEntryTypes(t *testing.T) {
	data := map[string]any{"flash": map[string]string(nil), "p": (*candProblem)(nil)}
	for _, test := range []struct {
		tmpl    string
		subject string
	}{
		{`{{.flash.success}}`, ""},
		{`{{.p.Title}}`, ""},
		{`{{.p.Titel}}`, "Titel"},
		{`{{.other.Title}}`, ""},
	} {
		tm := htmpl.Must(htmpl.New("t").Parse(test.tmpl))
		err := CheckHTMLWithCandidates(tm, data, []any{candProblem{}})
		if test.subject == "" {
			if err != nil {
				t.Errorf("%s: got %v, want no error", test.tmpl, err)
			}
			continue
		}
		var e *Error
		if !errors.As(err, &e) || e.Subject != test.subject {
			t.Errorf("%s: got %v, want an error about %q", test.tmpl, err, test.subject)
		}
	}
	// Without candidates, an entry's type is still used (ordinary checking).
	tm := htmpl.Must(htmpl.New("t").Parse(`{{.p.Titel}}`))
	var e *Error
	if err := CheckHTML(tm, data); !errors.As(err, &e) || e.Kind != ErrFieldNotFound {
		t.Errorf("CheckHTML: got %v, want ErrFieldNotFound", err)
	}
}

// FieldNames accepts fields of types that cannot be named, without a type.
// The value of such a field is again one of the candidates, as for any value
// of unknown type.
func TestCandidateFieldNames(t *testing.T) {
	ok := htmpl.Must(htmpl.New("t").Parse(`{{range .rows}}{{.HasAC}}{{.Problem.Title}}{{end}}`))
	if err := CheckHTMLWithCandidates(ok, map[string]any{}, []any{candProblem{}}); err == nil {
		t.Fatal("without FieldNames: got no error, want one about HasAC")
	}
	if err := CheckHTMLWithCandidates(ok, map[string]any{}, []any{candProblem{}, FieldNames{"HasAC", "Problem"}}); err != nil {
		t.Fatalf("with FieldNames: got %v, want no error", err)
	}
	bad := htmpl.Must(htmpl.New("t").Parse(`{{range .rows}}{{.Problem.Titel}}{{end}}`))
	var e *Error
	if err := CheckHTMLWithCandidates(bad, map[string]any{}, []any{candProblem{}, FieldNames{"Problem"}}); !errors.As(err, &e) || e.Subject != "Titel" {
		t.Fatalf("got %v, want an error about Titel", err)
	}
}

// The entry types of the data map describe the data only, not nested maps of the
// same type (reported by another user of this fork). They do apply to "$" and to
// templates invoked with ".".
func TestDataMapEntryTypesOnlyForTheData(t *testing.T) {
	data := map[string]any{"user": struct{ Name string }{}, "inner": map[string]any{}}
	for _, test := range []struct {
		tmpl    string
		wantErr bool
	}{
		{`{{.inner.user.Anything}}`, false},
		{`{{with .inner}}{{.user.Anything}}{{end}}`, false},
		{`{{range $k, $v := .inner}}{{$v}}{{end}}{{.user.Name}}`, false},
		{`{{index . "user"}}`, false},
		// The data itself, and its keys, can be passed to functions that take the data's type and strings.
		{`{{takesData .}}`, false},
		{`{{range $k, $v := .}}{{takesString $k}}{{end}}`, false},
		{`{{.user.Name}}`, false},
		{`{{$.user.Name}}`, false},
		{`{{define "p"}}{{.user.Name}}{{end}}{{template "p" .}}`, false},
		{`{{.user.Nmae}}`, true},
		{`{{$.user.Nmae}}`, true},
		{`{{define "p"}}{{.user.Nmae}}{{end}}{{template "p" .}}`, true},
	} {
		funcs := ttmpl.FuncMap{
			"takesData":   func(map[string]any) string { return "" },
			"takesString": func(string) string { return "" },
		}
		tm := ttmpl.Must(ttmpl.New("t").Funcs(funcs).Parse(test.tmpl))
		err := CheckText(tm, data)
		if test.wantErr != (err != nil) {
			t.Errorf("%s: got %v, want error %v", test.tmpl, err, test.wantErr)
		}
	}
}
