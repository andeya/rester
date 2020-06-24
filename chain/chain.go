// Package chain convert the method of nested fields in structure to call chain function.
//
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
//
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
		IsAborted() bool
	}
	// FindFunc find the first matched method
	FindFunc func(reflect.Method) (bool, error)
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
		recv      reflect.Type
		find      FindFunc
		methods   []methodFunc
		recvInfos []recvInfo
		recvTypes []reflect.Type
	}
	recvInfo struct {
		curOffset   uintptr
		curTypeElem reflect.Type
	}
)

// FindName finds the first method encountered that matches the methodName
func FindName(methodName string) FindFunc {
	return func(m reflect.Method) (bool, error) {
		if m.Name == methodName {
			return true, nil
		}
		return false, nil
	}
}

// ErrEmpty no method error
var ErrEmpty = errors.New("no method chain found")

// New creates a chained execution function.
// NOTE:
//  The method of specifying the name cannot have a return value
func New(obj NestedStruct, find FindFunc) (Func, error) {
	ctl := controller{
		recv: ameda.DereferenceImplementType(reflect.ValueOf(obj)),
		find: find,
	}
	err := ctl.makeMethods(0, ctl.recv)
	if err != nil {
		return nil, err
	}
	if len(ctl.methods) == 0 {
		return nil, ErrEmpty
	}
	return ctl.newChainFunc(), nil
}

func (c *controller) checkMethodName(methodName string) error {
	if !goutil.IsExportedName(methodName) {
		return fmt.Errorf("disallow unexported method name %s", methodName)
	}
	_, found := baseTypePtr.MethodByName(methodName)
	if found {
		return fmt.Errorf("disallow internally reserved method name %s", methodName)
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

func (c *controller) makeMethods(curOffset uintptr, curRecvElem reflect.Type) error {
	if !c.checkNestedStruct(curRecvElem) {
		return nil
	}
	curRecvPtr := ameda.ReferenceType(curRecvElem, 1)
	for i := curRecvPtr.NumMethod() - 1; i >= 0; i-- {
		m := curRecvPtr.Method(i)
		ok, err := c.checkMethod(m)
		if err != nil {
			return err
		}
		if !ok {
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
			if err := c.makeMethods(field.Offset, field.Type); err != nil {
				return err
			}
		}
	}
	return nil
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

func (c *controller) checkMethod(m reflect.Method) (bool, error) {
	ok, err := c.find(m)
	if !ok || err != nil {
		return false, err
	}
	err = c.checkMethodName(m.Name)
	if err != nil {
		return false, err
	}
	if m.Type.NumOut() > 0 {
		return false, fmt.Errorf("%s.%s has out parameters", m.Type.In(0).String(), m.Name)
	}
	return !goutil.IsCompositionMethod(m), nil
}

func (c *controller) checkNestedStruct(curRecv reflect.Type) bool {
	if curRecv.Kind() != reflect.Struct {
		return false
	}
	_, ok := reflect.New(curRecv).Interface().(NestedStruct)
	return ok
}
