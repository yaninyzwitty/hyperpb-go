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
	"github.com/bufbuild/fastpb/internal/tdp/thunks"
)

// CompileOption is a configuration setting for [Compile].
type CompileOption func(*compiler.Options)

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
	return thunks.SelectArchetype(fd, prof)
}

func (*backend) PopulateMethods(methods *protoiface.Methods) {
	methods.Unmarshal = unmarshalShim
	methods.CheckInitialized = requiredShim
}
