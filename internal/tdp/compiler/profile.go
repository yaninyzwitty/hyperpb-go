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

package compiler

import "google.golang.org/protobuf/reflect/protoreflect"

// Profile provides recorded profile information to [Compiler].
type Profile interface {
	// Field returns information about the given field site, if known.
	Field(site FieldSite) FieldProfile
}

// FieldSite is "call site" information for a message field. This type is the
// key used to look up information in a [Profile].
type FieldSite struct {
	// The field in question.
	Field protoreflect.FieldDescriptor
}

// FieldProfile is profiling information returned by a [Profile].
type FieldProfile struct {
	// How likely this field is to be seen on the wire, from 0 to 1.
	DecodeProbability float64
}

// DefaultProfile returns the default profile for a field.
//
// This essentially returns a "best guess" based on static information alone.
func (fs FieldSite) DefaultProfile() FieldProfile {
	var prof FieldProfile

	if fs.Field.IsExtension() {
		// Extensions default to being cold.
		prof.DecodeProbability = 0.25
	} else {
		prof.DecodeProbability = 0.5
	}

	return prof
}
