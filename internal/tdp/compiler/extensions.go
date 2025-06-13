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

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// ExtensionResolver provides a mechanism for retrieving the extensions associated with
// some message.
//
// This functionality is provided by [protoregistry.Types.RangeExtensionsByMessage],
// but there is no standard interface for this.
type ExtensionResolver interface {
	// FindExtensionsByMessage looks up all known extensions for the message with
	// the given name.
	FindExtensionsByMessage(name protoreflect.FullName) []protoreflect.ExtensionDescriptor
}

// ExtensionsFromRegistry wraps a [protoregistry.Types] to implement [ExtensionResolver].
type ExtensionsFromRegistry protoregistry.Types

// FindExtensionsByMessage implements [ExtensionResolver].
func (e *ExtensionsFromRegistry) FindExtensionsByMessage(
	name protoreflect.FullName,
) []protoreflect.ExtensionDescriptor {
	r := (*protoregistry.Types)(e)
	out := make([]protoreflect.ExtensionDescriptor, 0, r.NumExtensionsByMessage(name))
	r.RangeExtensionsByMessage(name, func(extn protoreflect.ExtensionType) bool {
		out = append(out, extn.TypeDescriptor().Descriptor())
		return true
	})
	return out
}
