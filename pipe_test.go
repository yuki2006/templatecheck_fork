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
)

// A value piped into a builtin function is its last argument. The checks for
// eq, ne, gt, len, not, index and slice must count it.
func TestPipedBuiltinArgs(t *testing.T) {
	type data struct {
		N int
		S string
		M map[string]int
		L []int
	}
	for _, test := range []struct {
		tmpl     string
		wantKind ErrorKind // ErrOther means no error
	}{
		{`{{if .N | ne 0}}x{{end}}`, ErrOther},
		{`{{if .N | eq 0}}x{{end}}`, ErrOther},
		{`{{if .N | gt 3}}x{{end}}`, ErrOther},
		{`{{.S | len}}`, ErrOther},
		{`{{if .N | not}}x{{end}}`, ErrOther},
		{`{{"k" | index .M}}`, ErrOther},
		{`{{1 | slice .L}}`, ErrOther},
		// Still an error: comparing with no other argument.
		{`{{if ne .N}}x{{end}}`, ErrWrongArgCount},
		// Still an error: an unorderable piped value.
		{`{{if .M | gt 3}}x{{end}}`, ErrComparison},
	} {
		tm := htmpl.Must(htmpl.New("t").Parse(test.tmpl))
		err := CheckHTML(tm, data{})
		if test.wantKind != ErrOther {
			var e *Error
			if !errors.As(err, &e) || e.Kind != test.wantKind {
				t.Errorf("%s: got %v, want kind %d", test.tmpl, err, test.wantKind)
			}
		} else if err != nil {
			t.Errorf("%s: got %v, want no error", test.tmpl, err)
		}
	}
}
