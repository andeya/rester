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
	Controller interface {
		chain.NestedStruct
		internal2(internalType)
		setContext(*fasthttp.RequestCtx)
		AbortWithReturn()
		BadRequest(code int, msg string)
		Forbidden(code int, msg string)
		InternalServerError(code int, msg string, err ...error)
		NotFound(code int, msg string)
		OK(value interface{})
		QueryAllArray(key string) []string
		Redirect(code int, location string)
		Unauthorized(code int, msg string)
	}
	BaseController struct {
		chain.Base
		*fasthttp.RequestCtx
	}
	// CodeMsg response body when the http code is not 200
	CodeMsg struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	internalType struct{}
)

var _ Controller = new(BaseController)
var binder = binding.New(nil).SetLooseZeroMode(true)

// MustNewHandlerFuncMap creates fasthttp.RequestHandler
func MustNewHandlerFuncMap(c Controller) map[string]fasthttp.RequestHandler {
	handlerMap, err := NewHandlerFuncMap(c)
	if err != nil {
		panic(err)
	}
	return handlerMap
}

type argsRequestCtx struct {
	*fasthttp.RequestCtx
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
	return func(m reflect.Method) (ok bool, err error) {
		ok, err = findMethod(m)
		if !ok && err == nil {
			ok, err = findAny(m)
		}
		if !ok || err != nil {
			return
		}
		if m.Type.NumIn() > 2 {
			return false, fmt.Errorf("%s.%s has more than two input parameters", m.Type.In(0).String(), m.Name)
		}
		return true, nil
	}
}

// NewHandlerFuncMap parses the Controller to generate a fasthttp.RequestHandler map with http method as the key
// NOTE: Any means all http methods
func NewHandlerFuncMap(c Controller) (map[string]fasthttp.RequestHandler, error) {
	handlers := make(map[string]fasthttp.RequestHandler)
	corsMethods := make(map[string]struct{})
	for _, httpMethod := range httpMethodList {
		fn, err := chain.New(c, newFinder(httpMethod))
		switch err {
		case nil:
			_, cors := splitMethod(httpMethod)
			handlers[httpMethod] = func(ctx *fasthttp.RequestCtx) {
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
			handlers[method] = func(c *fasthttp.RequestCtx) {
				corsFn(c)
				fn(c)
			}
		}
	}
	return handlers, nil
}

func newCorsFunc(corsMethods map[string]struct{}) fasthttp.RequestHandler {
	var a []string
	for m := range corsMethods {
		a = append(a, m)
	}
	allowMethods := strings.Join(a, ", ")
	return func(c *fasthttp.RequestCtx) {
		c.Request.Header.SetBytesV("Access-Control-Allow-Origin", c.Request.Header.Peek("Origin"))
		c.Request.Header.Set("Access-Control-Allow-Credentials", "true")
		c.Request.Header.Set("Access-Control-Allow-Methods", allowMethods)
		c.Request.Header.SetBytesV("Access-Control-Allow-Headers", c.Request.Header.Peek("Access-Control-Request-Headers"))
		if c.Request.Header.IsOptions() {
			c.Response.Reset()
			c.SetStatusCode(fasthttp.StatusNoContent)
			c.SetBodyString("")
		}
	}
}

func (*BaseController) internal2(internalType) {}

func (b *BaseController) setContext(c *fasthttp.RequestCtx) {
	b.RequestCtx = c
}

func (b BaseController) BadRequest(code int, msg string) {
	b.renderJSON(fasthttp.StatusBadRequest, CodeMsg{
		Code: code,
		Msg:  msg,
	})
	b.Abort(nil)
}

func (b BaseController) InternalServerError(code int, msg string, err ...error) {
	if len(err) > 0 && err[0] != nil {
		b.RequestCtx.Logger().Printf("msg=%s, error=%s", msg, err[0].Error())
	}
	b.renderJSON(fasthttp.StatusInternalServerError, CodeMsg{
		Code: code,
		Msg:  msg,
	})
	b.Abort(nil)
}

func (b BaseController) NotFound(code int, msg string) {
	b.renderJSON(fasthttp.StatusNotFound, CodeMsg{
		Code: code,
		Msg:  msg,
	})
	b.Abort(nil)
}

func (b BaseController) Unauthorized(code int, msg string) {
	b.renderJSON(fasthttp.StatusUnauthorized, CodeMsg{
		Code: code,
		Msg:  msg,
	})
	b.Abort(nil)
}

func (b BaseController) Forbidden(code int, msg string) {
	b.renderJSON(fasthttp.StatusForbidden, CodeMsg{
		Code: code,
		Msg:  msg,
	})
	b.Abort(nil)
}

func (b BaseController) Redirect(code int, location string) {
	b.RequestCtx.Redirect(location, code)
	b.Abort(nil)
}

func (b BaseController) OK(value interface{}) {
	b.renderJSON(fasthttp.StatusOK, value)
}

type returnFlag struct{}

func (b BaseController) AbortWithReturn() {
	b.Abort(nil)
	panic(returnFlag{})
}

func (b BaseController) Err() error {
	err := b.Base.Err()
	if err == nil {
		err = b.RequestCtx.Err()
	}
	return err
}

// QueryAllArray gets ["1","2","3","4","5"] from a=1,2,3&a=4&a=5
func (b BaseController) QueryAllArray(key string) []string {
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
func (b BaseController) IsAjaxRequest() bool {
	return ameda.UnsafeBytesToString(b.RequestCtx.Request.Header.Peek("X-Requested-With")) == "XMLHttpRequest"
}

func (b BaseController) renderJSON(code int, body interface{}) {
	renderJSON(b.RequestCtx, code, body)
}

var useTestMode = true

const jsonContentType = "application/json; charset=utf-8"

func renderJSON(ctx *fasthttp.RequestCtx, code int, body interface{}) {
	if useTestMode && goutil.IsGoTest() {
		b, _ := json.MarshalIndent(body, "", "  ")
		fmt.Printf("Respond: status_code=%d, json_body=%s\n", code, b)
		return
	}
	ctx.Response.Reset()
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
