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
	"testing"
	ttmpl "text/template"
)

func TestErrorFields(t *testing.T) {
	type S struct {
		A string
		B []int
	}
	for _, test := range []struct {
		contents    string
		wantKind    ErrorKind
		wantSubject string
		wantLine    int
		wantCol     int
		wantContext string
	}{
		{"x\n{{.X}}", ErrFieldNotFound, "X", 2, 2, ".X"},
		{"{{.A.Y}}", ErrFieldNotFound, "Y", 1, 4, ".A.Y"},
		{`{{template "nosuch"}}`, ErrTemplateNotDefined, "nosuch", 1, 11, `{{template "nosuch"}}`},
		{"{{nosuch}}", ErrOther, "", 0, 0, ""}, // reported by the parser, not the checker
		{"{{$x}}", ErrOther, "", 0, 0, ""},     // reported by the parser
		{"{{index .A 0 1}}", ErrIndex, "", 1, 13, "1"},
		{"{{len 1}}", ErrLen, "", 1, 6, "1"},
		{"{{range .A}}{{end}}", ErrNotIterable, "", 1, 8, ".A"},
		{`{{define "t"}}{{.A}}{{end}}{{template "t"}}`, ErrNilData, "A", 1, 16, ".A"},
	} {
		t.Run(test.contents, func(t *testing.T) {
			tmpl, err := ttmpl.New("t0").Parse(test.contents)
			if err != nil {
				if test.wantKind != ErrOther {
					t.Fatal(err)
				}
				return // parse errors are not *Error
			}
			err = CheckText(tmpl, S{})
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("got %v (%T), want *Error", err, err)
			}
			if e.Kind != test.wantKind || e.Subject != test.wantSubject {
				t.Errorf("kind, subject = %d, %q; want %d, %q", e.Kind, e.Subject, test.wantKind, test.wantSubject)
			}
			if e.ParseName != "t0" || e.Line != test.wantLine || e.Col != test.wantCol {
				t.Errorf("position = %s:%d:%d; want t0:%d:%d", e.ParseName, e.Line, e.Col, test.wantLine, test.wantCol)
			}
			if e.Context != test.wantContext {
				t.Errorf("context = %q, want %q", e.Context, test.wantContext)
			}
			if e.Reason == "" || e.Error() == "" {
				t.Errorf("empty reason or message: %+v", e)
			}
		})
	}
}

// The message is the same as before *Error existed.
func TestErrorMessage(t *testing.T) {
	tmpl := ttmpl.Must(ttmpl.New("t").Parse("x\n{{.X}}"))
	err := CheckText(tmpl, struct{}{})
	want := `template: t:2:2: checking "t" at <.X>: can't use field X in type struct {}`
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}
