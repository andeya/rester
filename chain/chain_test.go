package chain

import (
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

type T2 struct {
	T1
}

func (T2) M1() {
	fmt.Println("===== calling T2.M1")
}
func (*T2) M2(args string) {
	fmt.Printf("===== calling T2.M2, args=%s\n", args)
}
func (*T2) M3(t *testing.T, args int) {
	assert.Equal(t, 10, args)
	t.Logf("===== calling T2.M3, args=%d\n", args)
}

type Context struct {
	t *testing.T
}

func (c *Context) NewArg(idx int, in reflect.Type) (reflect.Value, error) {
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
		chain, err := New(obj, "M1")
		assert.NoError(t, err)
		err = chain(ctx)
		assert.NoError(t, err)
	}
	//
	_, err := New(new(T1), "Mn")
	assert.EqualError(t, err, "no method chain found")
	//
	chain, err := New(new(T2), "M2")
	assert.NoError(t, err)
	err = chain(ctx)
	assert.NoError(t, err)

	//
	chain, err = New(new(T2), "M3")
	assert.NoError(t, err)
	err = chain(ctx)
	assert.NoError(t, err)
}
