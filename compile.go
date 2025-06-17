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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoiface"
	"google.golang.org/protobuf/types/descriptorpb"

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

// WithExtensionsFromTypes uses a type registry to provide extension information
// about a message type.
func WithExtensionsFromTypes(types *protoregistry.Types) CompileOption {
	return func(c *compiler.Options) { c.Extensions = (*compiler.ExtensionsFromRegistry)(types) }
}

// WithExtensionsFromFiles uses a file registry to provide extension information
// about a message type.
func WithExtensionsFromFiles(files *protoregistry.Files) CompileOption {
	return func(c *compiler.Options) { c.Extensions = compiler.ExtensionsFromFile(files) }
}

// CompileFor is a helper for calling [Compile] using the descriptor of an
// existing message type.
//
// This is useful for getting a [Type] for a message type compiled into the
// binary. This will not work if T is some kind of dynamic type, like a
// *dynamicpb.Message, or a *fastpb.Message.
func CompileFor[T proto.Message](options ...CompileOption) *Type {
	// Allow the caller to override the extension registry by placing our
	// default registry first.
	options = append([]CompileOption{WithExtensionsFromTypes(protoregistry.GlobalTypes)}, options...)

	var m T
	return Compile(m.ProtoReflect().Descriptor(), options...)
}

// CompileFromBytes unmarshals a google.protobuf.FileDescriptorSet from schema,
// looks up a message with the given name, and compiles a type for it.
func CompileFromBytes(schema []byte, messageName protoreflect.FullName, options ...CompileOption) (*Type, error) {
	fds := new(descriptorpb.FileDescriptorSet)
	if err := proto.Unmarshal(schema, fds); err != nil {
		return nil, err
	}
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, err
	}
	desc, err := files.FindDescriptorByName(messageName)
	if err != nil {
		return nil, err
	}
	msgDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, protoregistry.NotFound
	}

	// Allow the caller to override the extension registry by placing our
	// default registry first.
	options = append([]CompileOption{WithExtensionsFromFiles(files)}, options...)
	return Compile(msgDesc, options...), nil
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
