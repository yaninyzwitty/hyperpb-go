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

// Package tdp contains the "object file format" used by hyperpb's parser.
// "TDP" stands for "table-driven parser".
//
// Subpackages of this package contain other components, like the parser
// and compiler.
//
// All fields in this package are exported because they are assembled and
// accessed by other internal packages. None of the types in this file should
// ever be exposed to users, either directly or as the type of an interface.
package tdp
