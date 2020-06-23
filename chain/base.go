package chain

import (
	"math"
	"reflect"
)

const abortIndex int8 = math.MaxInt8 / 2

type Base struct {
	argsFactory ArgsFactory
	recvs       []reflect.Value
	methods     []methodFunc
	index       int8
	abortError  error
}

var _ NestedStruct = new(Base)
var baseType = reflect.TypeOf(new(Base)).Elem()

func (Base) internal(internalType) {}

func (b *Base) init(argsFactory ArgsFactory, recvs []reflect.Value, methods []methodFunc) {
	b.argsFactory = argsFactory
	b.recvs = recvs
	b.methods = methods
}

func (b *Base) newArg(idx int, in reflect.Type) (reflect.Value, error) {
	return b.argsFactory.NewArg(idx, in)
}

func (b *Base) exec() error {
	b.index = -1
	b.Next()
	return b.Err()
}

// Next executes the pending methods in the chain inside the calling method.
func (b *Base) Next() {
	b.index++
	for b.index < int8(len(b.methods)) {
		b.methods[b.index](b, b.recvs[b.index])
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
