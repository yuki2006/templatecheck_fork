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
	"fmt"
	"strconv"
	"strings"
)

// ErrorKind classifies an *Error.
type ErrorKind int

const (
	// ErrOther is any error not covered by a more specific kind.
	ErrOther ErrorKind = iota
	// ErrFieldNotFound: the field or method does not exist on the type. Subject is the field name.
	ErrFieldNotFound
	// ErrUnexportedField: the field is unexported. Subject is the field name.
	ErrUnexportedField
	// ErrFieldHasArgs: arguments were given to a field or map element. Subject is the field name.
	ErrFieldHasArgs
	// ErrFieldOfUnknownType: a field was accessed on a value of unknown type (strict mode). Subject is the field name.
	ErrFieldOfUnknownType
	// ErrFieldOfInterface: a field was accessed on an interface value (strict mode). Subject is the field name.
	ErrFieldOfInterface
	// ErrNilData: a field was accessed on nil data, as in a template invoked
	// without a pipeline ({{template "x"}}). Subject is the field name.
	ErrNilData
	// ErrTemplateNotDefined: the invoked template does not exist. Subject is the template name.
	ErrTemplateNotDefined
	// ErrInconsistentTemplateTypes: a template is invoked with different types (strict mode). Subject is the template name.
	ErrInconsistentTemplateTypes
	// ErrFunctionNotDefined: the function does not exist. Subject is the function name.
	ErrFunctionNotDefined
	// ErrUndefinedVariable: the variable is not defined. Subject is the variable name, including the "$".
	ErrUndefinedVariable
	// ErrWrongArgCount: a function or method was called with the wrong number of arguments. Subject is its name.
	ErrWrongArgCount
	// ErrWrongArgType: an argument has the wrong type.
	ErrWrongArgType
	// ErrNotIterable: range over a value that cannot be iterated.
	ErrNotIterable
	// ErrIndex: invalid use of the index function.
	ErrIndex
	// ErrSlice: invalid use of the slice function.
	ErrSlice
	// ErrLen: invalid use of the len function.
	ErrLen
	// ErrComparison: invalid comparison (eq, ne, lt, le, gt, ge).
	ErrComparison
	// ErrIncompleteTemplate: the template is incomplete or empty. Subject is the template name.
	ErrIncompleteTemplate
	// ErrNoCandidateType: with candidate types (CheckHTMLWithCandidates), no candidate
	// type of a value of unknown type has the field or method. Subject is its name.
	ErrNoCandidateType
)

// Error is the type of the errors returned by the Check functions.
// Its message is the same as the one returned before this type existed.
type Error struct {
	Kind ErrorKind
	// Subject is the name the error is about (a field, template, function or
	// variable name), depending on Kind. It is empty if there is none.
	Subject string
	// TemplateName is the name of the template being checked when the error occurred.
	TemplateName string
	// ParseName, Line and Col give the position of the error, as reported by
	// parse.Tree.ErrorContext: the name of the top-level template whose text
	// contains the error, the 1-based line number, and the byte offset within
	// the line. All three are zero values if the position is unknown.
	ParseName string
	Line      int
	Col       int
	// Context is a textual representation of the node where the error occurred
	// (possibly abbreviated), as from parse.Tree.ErrorContext.
	Context string
	// Reason is the description of the error, without the location.
	Reason string

	msg string
}

func (e *Error) Error() string { return e.msg }

// errorKindf records an *Error of the given kind and terminates processing.
func (s *state) errorKindf(kind ErrorKind, subject string, format string, args ...any) {
	name := s.tmpl.Name()
	e := &Error{
		Kind:         kind,
		Subject:      subject,
		TemplateName: name,
		Reason:       fmt.Sprintf(format, args...),
	}
	if s.node == nil {
		e.msg = fmt.Sprintf("template: %s: %s", name, e.Reason)
	} else {
		location, context := s.tmpl.Tree().ErrorContext(s.node)
		e.Context = context
		e.ParseName, e.Line, e.Col = splitLocation(location)
		e.msg = fmt.Sprintf("template: %s: checking %q at <%s>: %s", location, name, context, e.Reason)
	}
	panic(checkError{e})
}

// splitLocation splits "name:line:col" as returned by parse.Tree.ErrorContext.
func splitLocation(location string) (name string, line, col int) {
	i := strings.LastIndex(location, ":")
	if i < 0 {
		return location, 0, 0
	}
	j := strings.LastIndex(location[:i], ":")
	if j < 0 {
		return location, 0, 0
	}
	line, err1 := strconv.Atoi(location[j+1 : i])
	col, err2 := strconv.Atoi(location[i+1:])
	if err1 != nil || err2 != nil {
		return location, 0, 0
	}
	return location[:j], line, col
}
