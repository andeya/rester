package chain

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type T1 struct {
	Base
}

func (T1) M1() {
	fmt.Println("===== calling T1.M1")
}

func (*T1) M2(args string) {
	fmt.Printf("===== calling T1.M2, args=%s\n", args)
}

func (t *T1) M4(args string) {
	fmt.Printf("===== calling T1.M4, args=%s\n", args)
	t.Abort(errors.New("T1.M4 test abort"))
}

func (t *T1) M5(test *testing.T, args string) {
	test.Logf("===== calling T1.M5 start, args=%s", args)
	t.Next()
	assert.False(test, t.IsAborted())
	test.Logf("===== calling T1.M5 end")
}

type T2 struct {
	T1
}

func (T2) M1(args string) {
	fmt.Printf("===== calling T2.M1, args=%s\n", args)
}

func (*T2) M2(args string) {
	fmt.Printf("===== calling T2.M2, args=%s\n", args)
}

func (*T2) M3(t *testing.T, args int) {
	assert.Equal(t, 10, args)
	t.Logf("===== calling T2.M3, args=%d\n", args)
}

func (*T2) M4(args string) {
	panic("should be aborted")
}

func (t *T2) M5(test *testing.T) {
	test.Logf("===== calling T2.M5")
}

type Context struct {
	t *testing.T
}

func (c *Context) Init(recv NestedStruct) error {
	fmt.Printf("Exec Struct: %T\n", recv)
	return nil
}

func (c *Context) Arg(recvType reflect.Type, idx int, in reflect.Type) (reflect.Value, error) {
	if idx == 0 && in.String() == "*testing.T" {
		return reflect.ValueOf(c.t), nil
	}
	if idx == 1 && in.String() == "int" {
		return reflect.ValueOf(10), nil
	}
	return reflect.ValueOf("test args"), nil
}

func TestNew(t *testing.T) {
	ctx := &Context{t}
	//
	for _, obj := range []NestedStruct{new(T1), new(T2)} {
		fn, err := New(obj, "M1")
		assert.NoError(t, err)
		err = fn(ctx)
		assert.NoError(t, err)
	}
	//
	_, err := New(new(T1), "Mn")
	assert.EqualError(t, err, "no method chain found")
	//
	fn, err := New(new(T2), "M2")
	assert.NoError(t, err)
	err = fn(ctx)
	assert.NoError(t, err)

	//
	fn, err = New(new(T2), "M3")
	assert.NoError(t, err)
	err = fn(ctx)
	assert.NoError(t, err)

	//
	fn, err = New(new(T1), "M4")
	assert.NoError(t, err)
	err = fn(ctx)
	assert.EqualError(t, err, "T1.M4 test abort")

	//
	fn, err = New(new(T2), "M4")
	assert.NoError(t, err)
	err = fn(ctx)
	assert.EqualError(t, err, "T1.M4 test abort")

	//
	fn, err = New(new(T2), "M5")
	assert.NoError(t, err)
	err = fn(ctx)
	assert.NoError(t, err)
}
