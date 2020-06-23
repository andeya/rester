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
	NestedStruct interface {
		internal(internalType)
		init(ArgsFactory, []reflect.Value, []methodFunc)
		newArg(idx int, in reflect.Type) (reflect.Value, error)
		exec() error
		Next()
		Abort(error)
		Err() error
	}
	ArgsFactory interface {
		NewArg(idx int, in reflect.Type) (reflect.Value, error)
	}
	ChainFunc    func(ArgsFactory) error
	methodFunc   func(base *Base, recv reflect.Value)
	internalType struct{}
	controller   struct {
		recv       reflect.Type
		methodName string
		methods    []methodFunc
		recvInfos  []recvInfo
	}
	recvInfo struct {
		curOffset uintptr
		curType   reflect.Type
		ptrNum    int
	}
)

// New creates a chained execution function.
// NOTE:
//  The method of specifying the name cannot have a return value
func New(obj NestedStruct, methodName string) (ChainFunc, error) {
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
		return nil, errors.New("no method chain found")
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

func (c *controller) newChainFunc() ChainFunc {
	return func(args ArgsFactory) error {
		topRecv, recvs := c.newRecvs()
		topRecv.init(args, recvs, c.methods)
		return topRecv.exec()
	}
}

func (c *controller) makeMethods(curOffset uintptr, curRecv reflect.Type) {
	if !c.checkNestedStruct(curRecv) {
		return
	}
	curRecvPtr := ameda.ReferenceType(curRecv, 1)
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
		ptrNum := 0
		recvType := inTypes[0]
		for {
			if recvType.Kind() != reflect.Ptr {
				break
			}
			recvType = recvType.Elem()
			ptrNum++
		}
		c.insertRecvInfo(curOffset, curRecv, ptrNum)
		fn := m.Func
		c.insertMethod(func(base *Base, recv reflect.Value) {
			inValues := make([]reflect.Value, numIn)
			inValues[0] = recv
			var err error
			for i := 1; i < numIn; i++ {
				inValues[i], err = base.newArg(i-1, inTypes[i]) // start at 0, skip receiver
				if err != nil {
					base.Abort(err)
					return
				}
			}
			fn.Call(inValues)
		})
		break // there can only be 1
	}
	for i := curRecv.NumField() - 1; i >= 0; i-- {
		field := curRecv.Field(i)
		if field.Anonymous {
			c.makeMethods(field.Offset, field.Type)
		}
	}
}

func (c *controller) insertRecvInfo(curOffset uintptr, curType reflect.Type, ptrNum int) {
	c.recvInfos = append(c.recvInfos, recvInfo{
		curOffset: curOffset,
		curType:   curType,
		ptrNum:    ptrNum,
	})
}

func (c *controller) newRecvs() (NestedStruct, []reflect.Value) {
	n := len(c.recvInfos)
	recvs := make([]reflect.Value, n)
	topRecv := reflect.New(c.recvInfos[0].curType)
	lastPtr := topRecv.Pointer()
	for i, info := range c.recvInfos {
		v := reflect.NewAt(info.curType, unsafe.Pointer(lastPtr+info.curOffset))
		lastPtr = v.Pointer()
		recvs[n-1-i] = ameda.ReferenceValue(v, info.ptrNum-1) // reverse
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
