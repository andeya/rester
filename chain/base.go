// Copyright 2020 HenryLee. All Rights Reserved.
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

package chain

import (
	"math"
	"reflect"
)

const abortIndex int8 = math.MaxInt8 / 2

type Base struct {
	args       Args
	ctl        *chainFactory
	recvs      []reflect.Value
	abortError error
	index      int8
}

var _ NestedStruct = new(Base)
var baseTypePtr = reflect.TypeOf(new(Base))

func (Base) internal(internalType) {}

func (b *Base) init(args Args, ctl *chainFactory, recvs []reflect.Value) {
	b.args = args
	b.ctl = ctl
	b.recvs = recvs
}

func (b *Base) newArg(recvType reflect.Type, idx int, in reflect.Type) (reflect.Value, error) {
	return b.args.Arg(recvType, idx, in)
}

func (b *Base) exec() error {
	b.index = -1
	b.Next()
	return b.abortError
}

// Next executes the pending methods in the chain inside the calling method.
func (b *Base) Next() {
	b.index++
	for b.index < int8(len(b.ctl.methods)) {
		b.ctl.methods[b.index](b, b.ctl.recvTypes[b.index], b.recvs[b.index])
		b.index++
	}
}

// Abort prevents pending methods from being called.
// NOTE:
//  That this will not stop the execution chain.
func (b *Base) Abort(err error) {
	b.abortError = err
	b.index = abortIndex
}

// IsAborted returns true if the current execution chain was aborted.
func (b *Base) IsAborted() bool {
	return b.index >= abortIndex
}
