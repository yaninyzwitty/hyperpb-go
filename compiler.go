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

package fastpb

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/bufbuild/fastpb/internal/tdp/compiler"
)

// CompileOption is a configuration setting for [Compile].
type CompileOption func(*compiler.Options)

// PGO adds profile-guided optimization information to a compiler.
//
// Profile is a function that returns profiling information for a given field.
// func PGO(prof func(site FieldSite) FieldProfile) CompileOption {
// 	return func(c *compiler) { c.prof = prof }
// }

// WithExtensions provides an extension resolver for a compiler.
//
// Unlike ordinary Protobuf parsers, fastpb does not perform extension
// resolution on the fly. Instead, any extensions that should be parsed must
// be provided up-front.
func WithExtensions(resolver compiler.ExtensionResolver) CompileOption {
	return func(c *compiler.Options) { c.Extensions = resolver }
}

// WithExtensionsFromRegistry uses a type registry to provide extension information
// about a message type.
func WithExtensionsFromRegistry(types *protoregistry.Types) CompileOption {
	return func(c *compiler.Options) { c.Extensions = (*compiler.ExtensionsFromRegistry)(types) }
}

// Compile compiles a descriptor into a [Type], for optimized parsing.
//
// Panics if md is too complicated (i.e. it exceeds internal limitations for the compiler).
func Compile(md protoreflect.MessageDescriptor, options ...CompileOption) *Type {
	opts := compiler.Options{
		Backend: (*backend)(nil),
	}

	for _, opt := range options {
		if opt != nil {
			opt(&opts)
		}
	}

	return newType(compiler.Compile(md, opts))
}

// backend implements the compiler backend interface.
type backend struct{}

func (*backend) SelectArchetype(fd protoreflect.FieldDescriptor, prof compiler.FieldProfile) *compiler.Archetype {
	var a *compiler.Archetype
	od := fd.ContainingOneof()
	switch {
	case fd.IsMap():
		k := fieldKind(fd.MapKey())
		v := fieldKind(fd.MapValue())
		a = mapFields[k][v]
	case fd.IsList():
		a = repeatedFields[fieldKind(fd)]
	case od != nil && od.Fields().Len() > 1:
		// One-element oneofs are treated like optional fields.
		a = oneofFields[fieldKind(fd)]
	case fd.HasPresence():
		a = optionalFields[fieldKind(fd)]
	default:
		a = singularFields[fieldKind(fd)]
	}

	return a
}

func (*backend) PopulateMethods(methods *protoiface.Methods) {
	methods.Unmarshal = unmarshalShim
	methods.CheckInitialized = requiredShim
}

// fieldKind extracts the field kind from a descriptor.
//
// This returns negative values for "custom" kinds.
func fieldKind(fd protoreflect.FieldDescriptor) protoreflect.Kind {
	switch k := fd.Kind(); k {
	case protoreflect.StringKind:
		fd2, ok := fd.(interface{ EnforceUTF8() bool })
		if fd.Syntax() == protoreflect.Proto3 || (ok && fd2.EnforceUTF8()) {
			return protoreflect.StringKind
		}
		return proto2StringKind

	default:
		return k
	}
}
