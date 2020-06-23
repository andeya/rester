package chain

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/henrylee2cn/ameda"
	"github.com/henrylee2cn/goutil"
)

type (
	// NestedStruct nested structure carrying method chain
	NestedStruct interface {
		internal(internalType)
		init(Args, *controller, []reflect.Value)
		newArg(recvType reflect.Type, idx int, in reflect.Type) (reflect.Value, error)
		exec() error
		Next()
		Abort(error)
		Err() error
	}
	// Args create input argument
	Args interface {
		Init(NestedStruct) error
		Arg(recvType reflect.Type, idx int, in reflect.Type) (reflect.Value, error)
	}
	// Func function to execute method chain
	Func         func(Args) error
	methodFunc   func(*Base, reflect.Type, reflect.Value)
	internalType struct{}
	controller   struct {
		recv       reflect.Type
		methodName string
		methods    []methodFunc
		recvInfos  []recvInfo
		recvTypes  []reflect.Type
	}
	recvInfo struct {
		curOffset   uintptr
		curTypeElem reflect.Type
	}
)

// ErrEmpty no method error
var ErrEmpty = errors.New("no method chain found")

// New creates a chained execution function.
// NOTE:
//  The method of specifying the name cannot have a return value
func New(obj NestedStruct, methodName string) (Func, error) {
	ctl := controller{
		recv:       ameda.DereferenceImplementType(reflect.ValueOf(obj)),
		methodName: methodName,
	}
	err := ctl.checkMethodName()
	if err != nil {
		return nil, err
	}
	ctl.makeMethods(0, ctl.recv)
	if len(ctl.methods) == 0 {
		return nil, ErrEmpty
	}
	return ctl.newChainFunc(), nil
}

func (c *controller) checkMethodName() error {
	if !goutil.IsExportedName(c.methodName) {
		return fmt.Errorf("disallow unexported method name %s", c.methodName)
	}
	_, found := baseType.MethodByName(c.methodName)
	if found {
		return fmt.Errorf("disallow internally reserved method name %s", c.methodName)
	}
	return nil
}

func (c *controller) newChainFunc() Func {
	return func(args Args) error {
		topRecv, recvs := c.newRecvs()
		err := args.Init(topRecv)
		if err != nil {
			return err
		}
		topRecv.init(args, c, recvs)
		return topRecv.exec()
	}
}

func (c *controller) makeMethods(curOffset uintptr, curRecvElem reflect.Type) {
	if !c.checkNestedStruct(curRecvElem) {
		return
	}
	curRecvPtr := ameda.ReferenceType(curRecvElem, 1)
	for i := curRecvPtr.NumMethod() - 1; i >= 0; i-- {
		m := curRecvPtr.Method(i)
		if !c.checkMethod(m) {
			continue
		}
		numIn := m.Type.NumIn()
		inTypes := make([]reflect.Type, numIn)
		for i := 0; i < numIn; i++ {
			inTypes[i] = m.Type.In(i)
		}
		ptrNum := 1
		if _, ok := curRecvElem.MethodByName(m.Name); ok {
			ptrNum = 0
		}
		c.insertRecvInfo(curOffset, curRecvElem, ameda.ReferenceType(curRecvElem, ptrNum))

		fn := m.Func
		c.insertMethod(func(base *Base, recvType reflect.Type, recvValue reflect.Value) {
			inValues := make([]reflect.Value, numIn)
			inValues[0] = recvValue
			var err error
			for i := 1; i < numIn; i++ {
				inValues[i], err = base.newArg(recvType, i-1, inTypes[i]) // start at 0, skip receiver
				if err != nil {
					base.Abort(err)
					return
				}
			}
			fn.Call(inValues)
		})
		break // there can only be 1
	}
	for i := curRecvElem.NumField() - 1; i >= 0; i-- {
		field := curRecvElem.Field(i)
		if field.Anonymous {
			c.makeMethods(field.Offset, field.Type)
		}
	}
}

func (c *controller) insertRecvInfo(curOffset uintptr, curTypeElem, curType reflect.Type) {
	c.recvInfos = append(c.recvInfos, recvInfo{
		curOffset:   curOffset,
		curTypeElem: curTypeElem,
	})
	c.recvTypes = append([]reflect.Type{curType}, c.recvTypes...) // reverse
}

func (c *controller) newRecvs() (NestedStruct, []reflect.Value) {
	n := len(c.recvInfos)
	recvs := make([]reflect.Value, n)
	topRecv := reflect.New(c.recvInfos[0].curTypeElem)
	lastPtr := topRecv.Pointer()
	for i, info := range c.recvInfos {
		v := reflect.NewAt(info.curTypeElem, unsafe.Pointer(lastPtr+info.curOffset))
		lastPtr = v.Pointer()
		recvs[n-1-i] = v // reverse
	}
	return topRecv.Interface().(NestedStruct), recvs
}

func (c *controller) insertMethod(m methodFunc) {
	c.methods = append([]methodFunc{m}, c.methods...) // reverse
}

func (c *controller) checkMethod(m reflect.Method) bool {
	return m.Name == c.methodName &&
		!goutil.IsCompositionMethod(m) &&
		m.Type.NumOut() == 0
}

func (c *controller) checkNestedStruct(curRecv reflect.Type) bool {
	if curRecv.Kind() != reflect.Struct {
		return false
	}
	_, ok := reflect.New(curRecv).Interface().(NestedStruct)
	return ok
}
