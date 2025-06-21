// Copyright 2025 Buf Technologies, Inc.
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

package debug

import (
	"fmt"
	"runtime"
	"strings"
)

// Unsupported returns "unimplemented" error for the calling function.
func Unsupported() error {
	pc, _, _, _ := runtime.Caller(1)
	return &errUnsupported{pc}
}

// errUnsupported is the error returned by Unimplemented.
type errUnsupported struct{ pc uintptr }

func (e *errUnsupported) Error() string {
	name := runtime.FuncForPC(e.pc).Name()
	if name == "" {
		return "hyperpb: unsupported operation"
	}

	slash := strings.LastIndexByte(name, '/')
	name = name[slash+1:]
	return fmt.Sprintf("hyperpb: %s() is not supported", name)
}
