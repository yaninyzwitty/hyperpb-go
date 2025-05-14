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

// Package fastpb is a highly optimized dynamic message library for Protobuf, which
// can generate parsers 2-3x faster than protobuf-go's generated code at
// runtime, using only reflection.
//
// To use this package, compile a [Type] using the [Compile] function. This is
// a one-time cost. The resulting value implements [protoreflect.MessageType].
//
// # Support Status
//
// This package does not implement roughly half of the protobuf-go reflection
// APIs, because it is specialized for parsing and reading only. More of the
// APIs may be implemented as time goes on. The following operations are not
// supported or not implemented:
//
//   - Unmarshaling onto an existing message.
//   - Clearing and mutating messages, including [protoreflect.Message].NewField
//     and message merging and cloning.
//   - Required field tracking (unmarshaling never fails due to missing fields).
//
// The following features are currently not implemented but there are plans
// to do so:
//
//   - Maps (currently they are rendered as repeated fields).
//   - Groups (groups will parse as unknown fields).
package fastpb
