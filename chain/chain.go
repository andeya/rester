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
		init(Args, *chainFactory, []reflect.Value)
		newArg(recvType reflect.Type, idx int, in reflect.Type) (reflect.Value, error)
		exec() error
		Next()
		Abort(error)
		IsAborted() bool
	}
	// FindFunc find the first matched method
	FindFunc func(level int, methods []reflect.Method) (*reflect.Method, error)
	// FactoryFunc creates a new NestedStruct object
	FactoryFunc func() NestedStruct
	// Args create input argument
	Args interface {
		Init(NestedStruct) error
		Arg(recvType reflect.Type, idx int, in reflect.Type) (reflect.Value, error)
	}
	// Func function to execute method chain
	Func         func(Args) error
	methodFunc   func(*Base, reflect.Type, reflect.Value)
	internalType struct{}
	chainFactory struct {
		recv        reflect.Type
		find        FindFunc
		methods     []methodFunc
		recvInfos   []recvInfo
		recvTypes   []reflect.Type
		factory     FactoryFunc
		recvPtrDiff int
	}
	recvInfo struct {
		curOffset   uintptr
		curTypeElem reflect.Type
	}
)

// FindName finds the first method encountered that matches the methodName
func FindName(methodName string) FindFunc {
	return func(_ int, methods []reflect.Method) (*reflect.Method, error) {
		for _, m := range methods {
			if m.Name == methodName {
				return &m, nil
			}
		}
		return nil, nil
	}
}

// ErrEmpty no method error
var ErrEmpty = errors.New("no method chain found")

// New creates a chained execution function.
// NOTE:
//  The method of specifying the name cannot have a return value
func New(obj NestedStruct, find FindFunc) (Func, error) {
	ctl := chainFactory{
		recv: ameda.DereferenceImplementType(reflect.ValueOf(obj)),
		find: find,
	}
	err := ctl.makeMethods(0, 0, ctl.recv)
	if err != nil {
		return nil, err
	}
	if len(ctl.methods) == 0 {
		return nil, ErrEmpty
	}
	return ctl.newChainFunc(), nil
}

// Make creates a chained execution function from NestedStruct factory.
// NOTE:
//  The method of specifying the name cannot have a return value
func Make(factory FactoryFunc, find FindFunc) (Func, error) {
	obj := factory()
	t := ameda.DereferenceInterfaceValue(reflect.ValueOf(obj)).Type()
	var recvPtrDiff int
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		recvPtrDiff--
	}
	recvPtrDiff++
	ctl := chainFactory{
		recv:        t,
		find:        find,
		factory:     factory,
		recvPtrDiff: recvPtrDiff,
	}
	err := ctl.makeMethods(0, 0, ctl.recv)
	if err != nil {
		return nil, err
	}
	if len(ctl.methods) == 0 {
		return nil, ErrEmpty
	}
	return ctl.newChainFunc(), nil
}

func (c *chainFactory) checkMethodName(methodName string) error {
	if !goutil.IsExportedName(methodName) {
		return fmt.Errorf("disallow unexported method name %q", methodName)
	}
	_, found := baseTypePtr.MethodByName(methodName)
	if found {
		return fmt.Errorf("disallow internally reserved method name %q", methodName)
	}
	return nil
}

func (c *chainFactory) newChainFunc() Func {
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

func (c *chainFactory) makeMethods(level int, curOffset uintptr, curRecvElem reflect.Type) error {
	if !c.checkNestedStruct(curRecvElem) {
		return nil
	}
	curRecvPtr := ameda.ReferenceType(curRecvElem, 1)
	numMethod := curRecvPtr.NumMethod()
	methods := make([]reflect.Method, 0, numMethod)
	for i := 0; i < numMethod; i++ {
		m := curRecvPtr.Method(i)
		if !goutil.IsCompositionMethod(m) {
			methods = append(methods, m)
		}
	}
	m, err := c.checkMethods(level, methods)
	if err != nil {
		return err
	}
	if m != nil {
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
	}
	for i := curRecvElem.NumField() - 1; i >= 0; i-- {
		field := curRecvElem.Field(i)
		if field.Anonymous {
			if err := c.makeMethods(level+1, field.Offset, field.Type); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *chainFactory) insertRecvInfo(curOffset uintptr, curTypeElem, curType reflect.Type) {
	c.recvInfos = append(c.recvInfos, recvInfo{
		curOffset:   curOffset,
		curTypeElem: curTypeElem,
	})
	c.recvTypes = append([]reflect.Type{curType}, c.recvTypes...) // reverse
}

func (c *chainFactory) newRecvs() (NestedStruct, []reflect.Value) {
	var topRecvObj NestedStruct
	var topRecvValue reflect.Value
	if c.factory == nil {
		topRecvValue = reflect.New(c.recvInfos[0].curTypeElem)
		topRecvObj = topRecvValue.Interface().(NestedStruct)
	} else {
		topRecvObj = c.factory()
		topRecvValue = ameda.ReferenceValue(ameda.DereferenceInterfaceValue(reflect.ValueOf(topRecvObj)), c.recvPtrDiff)
	}
	lastPtr := topRecvValue.Pointer()
	n := len(c.recvInfos)
	recvs := make([]reflect.Value, n)
	for i, info := range c.recvInfos {
		v := reflect.NewAt(info.curTypeElem, unsafe.Pointer(lastPtr+info.curOffset))
		lastPtr = v.Pointer()
		recvs[n-1-i] = v // reverse
	}
	return topRecvObj, recvs
}

func (c *chainFactory) insertMethod(m methodFunc) {
	c.methods = append([]methodFunc{m}, c.methods...) // reverse
}

func (c *chainFactory) checkMethods(level int, methods []reflect.Method) (*reflect.Method, error) {
	m, err := c.find(level, methods)
	if err != nil || m == nil {
		return m, err
	}
	err = c.checkMethodName(m.Name)
	if err != nil {
		return nil, err
	}
	if m.Type.NumOut() > 0 {
		return nil, fmt.Errorf("%s.%s has out parameters", m.Type.In(0).String(), m.Name)
	}
	return m, nil
}

func (c *chainFactory) checkNestedStruct(curRecv reflect.Type) bool {
	if curRecv.Kind() != reflect.Struct {
		return false
	}
	_, ok := reflect.New(curRecv).Interface().(NestedStruct)
	return ok
}
