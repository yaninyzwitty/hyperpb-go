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
	"path"
	"runtime"
	"strings"
)

// Stack is like [runtime/debug.Stack], but with a skip parameter and an
// easier to read format.
func Stack(skip int) string {
	var out strings.Builder

	trace := make([]uintptr, 32)
	for {
		n := runtime.Callers(skip, trace)
		if n < len(trace) {
			trace = trace[:n]
			break
		}
		trace = make([]uintptr, len(trace)*2)
	}

	frames := runtime.CallersFrames(trace)
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&out,
			"- %-24v 0x%x+0x%-4x %v:%v\n",
			path.Base(frame.Function)+"()", frame.Entry, frame.PC-frame.Entry,
			frame.File, frame.Line,
		)

		if !more {
			break
		}
	}

	return out.String()
}
