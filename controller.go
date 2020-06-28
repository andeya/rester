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

package rester

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/bytedance/json"
	"github.com/henrylee2cn/ameda"
	"github.com/henrylee2cn/goutil"
	"github.com/valyala/fasthttp"

	"github.com/henrylee2cn/rester/binding"
	"github.com/henrylee2cn/rester/chain"
)

type (
	// Controller chain of middleware and handler defined by struct
	Controller interface {
		chain.NestedStruct
		internal2(internalType)
		setContext(*RequestCtx)
		BadRequest(code int, msg string)
		Forbidden(code int, msg string)
		InternalServerError(code int, msg string, err ...error)
		NotFound(...string)
		NotModified()
		OK(value interface{})
		QueryAllArray(key string) []string
		Redirect(code int, location string)
		Unauthorized(code int, msg string)
	}
	// BaseCtl the base controller that each controller must inherit
	BaseCtl struct {
		chain.Base
		*RequestCtx
	}
	// CodeMsg response body when the http code is not 200
	CodeMsg struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	// H is a shortcut for map[string]interface{}
	H            map[string]interface{}
	internalType struct{}
)

var _ Controller = new(BaseCtl)
var binder = binding.New(nil).SetLooseZeroMode(true)

// MustMakeHandlers creates map {httpMethod:RequestHandler} from the Controller factory.
// NOTE:
//  panic when something goes wrong
func MustMakeHandlers(factory func() Controller) map[string]RequestHandler {
	handlers, err := MakeHandlers(factory)
	checkNewChainErr(err)
	return handlers
}

// MustNewHandlers converts the Controller to map {httpMethod:RequestHandler}.
// NOTE:
//  panic when something goes wrong
func MustNewHandlers(c Controller) map[string]RequestHandler {
	handlers, err := NewHandlers(c)
	checkNewChainErr(err)
	return handlers
}

func checkNewChainErr(err error) {
	if err != nil && err != chain.ErrEmpty {
		panic(err)
	}
}

// MakeHandlers creates map {httpMethod:RequestHandler} from the Controller factory.
// NOTE: Any means all http methods
func MakeHandlers(factory func() Controller) (map[string]RequestHandler, error) {
	return newHandlers(nil, factory)
}

// NewHandlers converts the Controller to map {httpMethod:RequestHandler}.
// NOTE: Any means all http methods
func NewHandlers(c Controller) (map[string]RequestHandler, error) {
	return newHandlers(c, nil)
}

func newHandlers(c Controller, factory func() Controller) (map[string]RequestHandler, error) {
	handlers := make(map[string]RequestHandler)
	corsMethods := make(map[string]struct{})
	if factory != nil {
		c = factory()
	}
	var err error
	for _, httpMethod := range httpMethodList {
		var fn chain.Func
		if factory != nil {
			fn, err = chain.Make(func() chain.NestedStruct {
				return factory()
			}, newFinder(httpMethod))
		} else {
			fn, err = chain.New(c, newFinder(httpMethod))
		}
		switch err {
		case nil:
			var cors bool
			httpMethod, cors = splitMethod(httpMethod)
			handlers[httpMethod] = func(ctx *RequestCtx) {
				err := fn(argsRequestCtx{ctx})
				switch e := err.(type) {
				case nil:
				case *CodeMsg:
					if e.Code >= 400 && e.Code < 600 {
						renderJSON(ctx, e.Code, e)
					}
				default:
					renderJSON(ctx, fasthttp.StatusInternalServerError, e)
				}
			}
			if cors {
				corsMethods[httpMethod] = struct{}{}
			}
		case chain.ErrEmpty:
		default:
			return nil, err
		}
	}

	if len(handlers) == 0 {
		return nil, fmt.Errorf("%T has no method with the same name as HTTP method, eg. GET or CORS_GET", c)
	}

	if len(corsMethods) > 0 {
		corsFn := newCorsFunc(corsMethods)
		if handlers["OPTIONS"] == nil {
			handlers["OPTIONS"] = corsFn
		}
		for method := range corsMethods {
			fn := handlers[method]
			handlers[method] = func(c *RequestCtx) {
				corsFn(c)
				fn(c)
			}
		}
	}
	return handlers, nil
}
func newCorsFunc(corsMethods map[string]struct{}) RequestHandler {
	var a []string
	for m := range corsMethods {
		a = append(a, m)
	}
	allowMethods := strings.Join(a, ", ")
	return func(c *RequestCtx) {
		c.Response.Header.SetBytesV("Access-Control-Allow-Origin", c.Request.Header.Peek("Origin"))
		c.Response.Header.Set("Access-Control-Allow-Credentials", "true")
		c.Response.Header.Set("Access-Control-Allow-Methods", allowMethods)
		c.Response.Header.SetBytesV("Access-Control-Allow-Headers", c.Request.Header.Peek("Access-Control-Request-Headers"))
		if c.Request.Header.IsOptions() {
			c.SetStatusCode(fasthttp.StatusNoContent)
			c.SetBodyString("")
		}
	}
}

func (*BaseCtl) internal2(internalType) {}

func (b *BaseCtl) setContext(c *RequestCtx) {
	b.RequestCtx = c
}

func (b BaseCtl) BadRequest(code int, msg string) {
	b.renderJSON(fasthttp.StatusBadRequest, CodeMsg{
		Code: code,
		Msg:  msg,
	})
	b.Abort(nil)
}

func (b BaseCtl) InternalServerError(code int, msg string, err ...error) {
	if len(err) > 0 && err[0] != nil {
		b.RequestCtx.Logger().Printf("msg=%s, error=%s", msg, err[0].Error())
	}
	b.renderJSON(fasthttp.StatusInternalServerError, CodeMsg{
		Code: code,
		Msg:  msg,
	})
	b.Abort(nil)
}

func (b BaseCtl) NotFound(msg ...string) {
	msg = append(msg, "404 Page not found")
	ctx := b.RequestCtx
	ctx.Response.Reset()
	ctx.SetStatusCode(fasthttp.StatusNotFound)
	ctx.SetBodyString(msg[0])
	b.Abort(nil)
}

// NotModified resets response and sets '304 Not Modified' response status code.
func (b BaseCtl) NotModified() {
	b.RequestCtx.NotModified()
	b.Abort(nil)
}

func (b BaseCtl) Unauthorized(code int, msg string) {
	b.renderJSON(fasthttp.StatusUnauthorized, CodeMsg{
		Code: code,
		Msg:  msg,
	})
	b.Abort(nil)
}

func (b BaseCtl) Forbidden(code int, msg string) {
	b.renderJSON(fasthttp.StatusForbidden, CodeMsg{
		Code: code,
		Msg:  msg,
	})
	b.Abort(nil)
}

func (b BaseCtl) Redirect(code int, location string) {
	b.RequestCtx.Redirect(location, code)
	b.Abort(nil)
}

func (b BaseCtl) OK(value interface{}) {
	b.renderJSON(fasthttp.StatusOK, value)
}

// QueryAllArray gets ["1","2","3","4","5"] from a=1,2,3&a=4&a=5
func (b BaseCtl) QueryAllArray(key string) []string {
	if b.RequestCtx == nil {
		return nil
	}
	r := make([]string, 0, 4)
	b.RequestCtx.QueryArgs().VisitAll(func(k, v []byte) {
		if key == ameda.UnsafeBytesToString(k) {
			r = append(r, strings.Split(ameda.UnsafeBytesToString(v), ",")...)
		}
	})
	return r
}

// IsAjaxRequest front-end setup required
func (b BaseCtl) IsAjaxRequest() bool {
	return ameda.UnsafeBytesToString(b.RequestCtx.Request.Header.Peek("X-Requested-With")) == "XMLHttpRequest"
}

func (b BaseCtl) renderJSON(code int, body interface{}) {
	renderJSON(b.RequestCtx, code, body)
}

var useTestMode = true

const jsonContentType = "application/json; charset=utf-8"

func renderJSON(ctx *RequestCtx, code int, body interface{}) {
	if useTestMode && goutil.IsGoTest() {
		b, _ := json.MarshalIndent(body, "", "  ")
		fmt.Printf("Respond: status_code=%d, json_body=%s\n", code, b)
		return
	}
	ctx.SetContentType(jsonContentType)
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		bodyBytes, _ = json.Marshal(CodeMsg{
			Code: fasthttp.StatusInternalServerError,
			Msg:  err.Error(),
		})
		ctx.SetBody(bodyBytes)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(bodyBytes)
}

var _ error = new(CodeMsg)

func (c *CodeMsg) Error() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("code=%d, msg=%s", c.Code, c.Msg)
}

type argsRequestCtx struct {
	*RequestCtx
}

func (a argsRequestCtx) Init(recv chain.NestedStruct) error {
	c := recv.(Controller)
	c.setContext(a.RequestCtx)
	return nil
}

func (a argsRequestCtx) Arg(recvType reflect.Type, idx int, in reflect.Type) (reflect.Value, error) {
	var ptrNum int
	for in.Kind() == reflect.Ptr {
		in = in.Elem()
		ptrNum++
	}
	vPtr := reflect.New(in)
	reqRecvPtr := vPtr.Interface()
	err := binder.BindAndValidate(reqRecvPtr, a.RequestCtx)
	if err != nil {
		return reflect.Value{}, &CodeMsg{400, err.Error()}
	}
	return ameda.ReferenceValue(vPtr, ptrNum-1), nil
}

func newFinder(httpMethod string) chain.FindFunc {
	findMethod := chain.FindName(httpMethod)
	findAny := chain.FindName(anyMethod)
	return func(level int, methods []reflect.Method) (m *reflect.Method, err error) {
		m, err = findMethod(level, methods)
		if m == nil && err == nil {
			if level == 0 {
				return nil, chain.ErrEmpty
			}
			m, err = findAny(level, methods)
		}
		if m == nil || err != nil {
			return
		}
		if m.Type.NumIn() > 2 {
			return nil, fmt.Errorf("%s.%s has more than two input parameters", m.Type.In(0).String(), m.Name)
		}
		return m, nil
	}
}
