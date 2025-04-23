// Copyright 2020-2025 Buf Technologies, Inc.
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

package fastpb

import (
	"errors"
	"fmt"
	"io"
)

const (
	errCodeOk errCode = iota
	// These match the errors in protowire.
	errCodeTruncated
	errCodeFieldNumber
	errCodeOverflow
	errCodeReserved
	errCodeEndGroup
	errCodeRecursionDepth

	errCodeUTF8
)

type errCode int

var errs = [...]error{
	errCodeOk:          nil,
	errCodeTruncated:   io.ErrUnexpectedEOF,
	errCodeFieldNumber: errors.New("invalid field number"),
	errCodeOverflow:    errors.New("variable length integer overflow"),
	errCodeReserved:    errors.New("cannot parse reserved wire type"),
	errCodeEndGroup:    errors.New("mismatching end group marker"),
	errCodeUTF8:        errors.New("invalid UTF-8 in string"),
}

// errParse is an error returned by the TDP parser.
type errParse struct {
	code   errCode
	offset int
}

// Offset returns the offset at which the error occurred.
func (e *errParse) Offset() int {
	return e.offset
}

// Unwrap implements error unwrapping viz [errors.Unwrap].
func (e *errParse) Unwrap() error {
	return errs[e.code]
}

// Error implements [error].
func (e *errParse) Error() string {
	return fmt.Sprintf("fastpb: parser error at offset %d/%#x: %v", e.offset, e.offset, e.Unwrap())
}
