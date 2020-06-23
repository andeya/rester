package chain

import (
	"math"
	"reflect"
)

const abortIndex int8 = math.MaxInt8 / 2

type Base struct {
	args       Args
	ctl        *controller
	recvs      []reflect.Value
	abortError error
	index      int8
}

var _ NestedStruct = new(Base)
var baseTypePtr = reflect.TypeOf(new(Base))

func (Base) internal(internalType) {}

func (b *Base) init(args Args, ctl *controller, recvs []reflect.Value) {
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
	return b.Err()
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

// Err returns the execution error.
func (b *Base) Err() error {
	return b.abortError
}
