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

package tdp

import "google.golang.org/protobuf/reflect/protoreflect"

// Scalar is a Protobuf scalar type.
type Scalar interface {
	int32 | int64 |
		uint32 | uint64 |
		float32 | float64 |
		protoreflect.EnumNumber | bool
}

// Int is any of the integer types that this package has to handle
// generically.
type Int interface {
	~int8 | ~uint8 | ~int32 | ~int64 | ~uint32 | ~uint64
}

// Number is anything from [Int], or a float.
type Number interface {
	Int | ~float32 | ~float64
}
