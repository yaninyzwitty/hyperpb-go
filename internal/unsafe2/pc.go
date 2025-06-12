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

package unsafe2

// PC is a raw function pointer, which can be used to store captureless
// funcs.
//
// Suppose a func() is in rax. Go implements calling it by emitting the
// following code:
//
//	mov  rdx, rax
//	mov  rcx, [rdx]
//	call rcx
//
// For a captureless func, this load will be of a constant containing the PC
// of the function to call. This can result in cache misses. This type works
// around that by keeping the PC local, so the resulting load avoids this
// problem.
type PC[F any] uintptr

// NewPC wraps a func. This performs no checking that the func does not
// capture any variables.
func NewPC[F any](f F) PC[F] {
	// Recall that a func()'s layout is *runtime.funcval, and PC[F] is emulating
	// runtime.funcval.
	return *BitCast[*PC[F]](f)
}

// Get returns the func this PC wraps.
func (pc *PC[F]) Get() F {
	return BitCast[F](pc)
}
