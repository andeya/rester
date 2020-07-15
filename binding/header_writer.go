package binding

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/go-tagexpr"
	"github.com/henrylee2cn/goutil"
	"github.com/henrylee2cn/goutil/tpack"
	"github.com/valyala/fasthttp"
)

// HeaderWriter class for writing header and cookie
type HeaderWriter struct {
	headersBinders map[int32]*headersBinder
	lock           sync.RWMutex
	tagNames       TagNames
	vm             *tagexpr.VM
}

var defaultHeaderWriter = NewHeaderWriter(&TagNames{
	SetHeader: "header",
	SetCookie: "cookie",
})

// WriteHeader writes header and cookie to headersBinder,
// according to the 'header' and 'cookie' struct tags.
func WriteHeader(c *fasthttp.RequestCtx, result interface{}) error {
	return defaultHeaderWriter.Write(c, result)
}

// TagNames struct tag naming
type TagNames struct {
	// SetHeader use 'header' by default when empty
	SetHeader string
	// SetCookie use 'cookie' by default when empty
	SetCookie string
}

// NewHeaderWriter creates *HeaderWriter object.
func NewHeaderWriter(tagNames *TagNames) *HeaderWriter {
	if tagNames == nil {
		tagNames = new(TagNames)
	}
	goutil.InitAndGetString(&tagNames.SetHeader, "header")
	goutil.InitAndGetString(&tagNames.SetCookie, "cookie")
	return &HeaderWriter{
		tagNames:       *tagNames,
		vm:             tagexpr.New(),
		headersBinders: make(map[int32]*headersBinder, 1024),
	}
}

// Write writes header and cookie from result to w.
func (r *HeaderWriter) Write(w *fasthttp.RequestCtx, result interface{}) error {
	if result == nil {
		return nil
	}
	v, err := r.structValueOf(result)
	if err != nil || !v.IsValid() {
		return nil
	}
	binder, err := r.getOrPrepareBinder(v)
	if binder == nil {
		return err
	}
	expr, err := r.vm.Run(result)
	if err != nil {
		return err
	}
	for _, c := range binder.cookies {
		if err = c.SetCookie(w, expr); err != nil {
			return err
		}
	}
	for _, h := range binder.headers {
		h.AddHeader(w, expr)
	}
	return nil
}

func (r *HeaderWriter) getOrPrepareBinder(value reflect.Value) (*headersBinder, error) {
	runtimeTypeID := tpack.From(value).RuntimeTypeID()
	r.lock.RLock()
	binder, ok := r.headersBinders[runtimeTypeID]
	r.lock.RUnlock()
	if ok {
		return binder, nil
	}
	expr, err := r.vm.Run(reflect.New(value.Type()).Elem())
	if err != nil {
		return nil, err
	}
	binder = newHeaderBinder()
	expr.RangeFields(func(fh *tagexpr.FieldHandler) bool {
		field := fh.StructField()
		fs := fh.StringSelector()
		err = binder.setHeaderBinder(field, fs, r.tagNames.SetHeader)
		if err != nil {
			return false
		}
		err = binder.setCookieBinder(field, fs, r.tagNames.SetCookie)
		if err != nil {
			return false
		}
		return true
	})
	if binder.isEmpty() {
		binder = nil
	}
	r.lock.Lock()
	r.headersBinders[runtimeTypeID] = binder
	r.lock.Unlock()
	return binder, err
}

func cleanTagValue(tagVal string) string {
	tagVal = strings.TrimSpace(tagVal)
	tagVal = strings.Replace(tagVal, " ", "", -1)
	tagVal = strings.Replace(tagVal, "\t", "", -1)
	return tagVal
}

func splitRequired(tagVal string) (string, bool) {
	name := strings.TrimSuffix(tagVal, ",required")
	return name, name != tagVal
}

type headersBinder struct {
	cookies map[string]*cookieBinder // <name,cookie>
	headers map[string]*headerBinder // <name,header>
}

func (r *headersBinder) setHeaderBinder(field reflect.StructField, fs, tagName string) error {
	tagVal, ok := field.Tag.Lookup(tagName)
	if !ok {
		return nil
	}
	if err := checkString(field.Name, field.Type); err != nil {
		return err
	}
	name, required := splitRequired(cleanTagValue(tagVal))
	if name == "" {
		name = field.Name
	}
	r.headers[name] = &headerBinder{
		name:          name,
		valueFS:       fs,
		valueRequired: required,
	}
	return nil
}

func (r *headersBinder) setCookieBinder(field reflect.StructField, fs, tagName string) error {
	tagVal, ok := field.Tag.Lookup(tagName)
	if !ok {
		return nil
	}
	name, required := splitRequired(cleanTagValue(tagVal))
	if name == "" {
		name = field.Name
	}
	a := strings.SplitN(name, ",", 2)
	name = a[0]
	c, ok := r.cookies[name]
	if !ok {
		c = &cookieBinder{
			name: name,
		}
		r.cookies[name] = c
	}
	var pos string
	if len(a) == 2 {
		pos = a[1]
	}
	switch strings.ToLower(pos) {
	case "path":
		if err := checkString(field.Name, field.Type); err != nil {
			return err
		}
		c.pathFS = fs
		c.pathRequired = required
	case "domain":
		if err := checkString(field.Name, field.Type); err != nil {
			return err
		}
		c.domainFS = fs
		c.domainRequired = required
	case "expires":
		if err := checkString(field.Name, field.Type); err != nil {
			if err = checkTime(field.Name, field.Type); err != nil {
				return err
			}
		}
		c.expiresFS = fs
		c.expiresRequired = required
	case "maxage":
		if err := checkInt(field.Name, field.Type); err != nil {
			return err
		}
		c.maxAgeFS = fs
		c.maxAgeRequired = required
	case "secure":
		if err := checkBool(field.Name, field.Type); err != nil {
			return err
		}
		c.secureFS = fs
		c.secureRequired = required
	case "httponly":
		if err := checkBool(field.Name, field.Type); err != nil {
			return err
		}
		c.httpOnlyFS = fs
		c.httpOnlyRequired = required
	case "samesite":
		if err := checkInt(field.Name, field.Type); err != nil {
			return err
		}
		c.sameSiteFS = fs
		c.sameSiteRequired = required
	case "value":
		fallthrough
	default: // Value
		if err := checkString(field.Name, field.Type); err != nil {
			return err
		}
		c.valueFS = fs
		c.valueRequired = required
	}
	return nil
}

func (r *HeaderWriter) structValueOf(value interface{}) (reflect.Value, error) {
	v, ok := value.(reflect.Value)
	if !ok {
		v = reflect.ValueOf(value)
	}
	v = goutil.DereferenceValue(v)
	if v.Kind() != reflect.Struct {
		return v, fmt.Errorf("type %T is not a non-nil struct", value)
	}
	return v, nil
}

func newHeaderBinder() *headersBinder {
	return &headersBinder{
		cookies: make(map[string]*cookieBinder, 32),
		headers: make(map[string]*headerBinder, 32),
	}
}

func (h *headersBinder) isEmpty() bool {
	if h == nil {
		return true
	}
	if len(h.cookies) == 0 && len(h.headers) == 0 {
		return true
	}
	return false
}

type headerBinder struct {
	name          string
	valueFS       string
	valueRequired bool
}

func (h *headerBinder) AddHeader(w *fasthttp.RequestCtx, tagExpr *tagexpr.TagExpr) error {
	v := getString(tagExpr, h.valueFS)
	if v == "" && h.valueRequired {
		return fmt.Errorf("the header %s missing required value", h.name)
	}
	w.Response.Header.Add(h.name, v)
	return nil
}

type cookieBinder struct {
	name string

	valueFS    string
	pathFS     string
	domainFS   string
	expiresFS  string
	maxAgeFS   string
	secureFS   string
	httpOnlyFS string
	sameSiteFS string

	valueRequired    bool
	pathRequired     bool
	domainRequired   bool
	expiresRequired  bool
	maxAgeRequired   bool
	secureRequired   bool
	httpOnlyRequired bool
	sameSiteRequired bool
}

func (c *cookieBinder) SetCookie(w *fasthttp.RequestCtx, tagExpr *tagexpr.TagExpr) (err error) {
	ck := fasthttp.AcquireCookie()
	defer func() {
		if err != nil {
			fasthttp.ReleaseCookie(ck)
		} else {
			w.Response.Header.SetCookie(ck)
		}
	}()
	c.setName(ck)
	if err = c.setValue(ck, tagExpr); err != nil {
		return err
	}
	if err = c.setPath(ck, tagExpr); err != nil {
		return err
	}
	if err = c.setDomain(ck, tagExpr); err != nil {
		return err
	}
	if err = c.setExpires(ck, tagExpr); err != nil {
		return err
	}
	if err = c.setMaxAge(ck, tagExpr); err != nil {
		return err
	}
	if err = c.setSecure(ck, tagExpr); err != nil {
		return err
	}
	if err = c.setHttpOnly(ck, tagExpr); err != nil {
		return err
	}
	if err = c.setSameSite(ck, tagExpr); err != nil {
		return err
	}
	return nil
}

func (c *cookieBinder) setName(ck *fasthttp.Cookie) {
	ck.SetKey(c.name)
}

func (c *cookieBinder) setValue(ck *fasthttp.Cookie, tagExpr *tagexpr.TagExpr) error {
	v := url.QueryEscape(getString(tagExpr, c.valueFS))
	if v == "" && c.valueRequired {
		return fmt.Errorf("the cookie %s missing required Value field", c.name)
	}
	ck.SetValue(v)
	return nil
}

func (c *cookieBinder) setPath(ck *fasthttp.Cookie, tagExpr *tagexpr.TagExpr) error {
	v := getString(tagExpr, c.pathFS)
	if v == "" {
		if c.pathRequired {
			return fmt.Errorf("the cookie %s missing required Path field", c.name)
		}
		v = "/"
	}
	ck.SetPath(v)
	return nil
}

func (c *cookieBinder) setDomain(ck *fasthttp.Cookie, tagExpr *tagexpr.TagExpr) error {
	v := getString(tagExpr, c.domainFS)
	if v == "" && c.domainRequired {
		return fmt.Errorf("the cookie %s missing required Domain field", c.name)
	}
	ck.SetDomain(v)
	return nil
}

func (c *cookieBinder) setExpires(ck *fasthttp.Cookie, tagExpr *tagexpr.TagExpr) error {
	v, err := getTime(tagExpr, c.expiresFS)
	if err != nil {
		return err
	}
	if v == (time.Time{}) && c.expiresRequired {
		return fmt.Errorf("the cookie %s missing required Expires field", c.name)
	}
	ck.SetExpire(v)
	return nil
}

func (c *cookieBinder) setMaxAge(ck *fasthttp.Cookie, tagExpr *tagexpr.TagExpr) error {
	v := getInt(tagExpr, c.maxAgeFS)
	if v == 0 && c.maxAgeRequired {
		return fmt.Errorf("the cookie %s missing required MaxAge field", c.name)
	}
	ck.SetMaxAge(v)
	return nil
}

func (c *cookieBinder) setSecure(ck *fasthttp.Cookie, tagExpr *tagexpr.TagExpr) error {
	v, valid := getBool(tagExpr, c.secureFS)
	if !valid && c.secureRequired {
		return fmt.Errorf("the cookie %s missing required Secure field", c.name)
	}
	ck.SetSecure(v)
	return nil
}

func (c *cookieBinder) setHttpOnly(ck *fasthttp.Cookie, tagExpr *tagexpr.TagExpr) error {
	v, valid := getBool(tagExpr, c.httpOnlyFS)
	if !valid && c.httpOnlyRequired {
		return fmt.Errorf("the cookie %s missing required HttpOnly field", c.name)
	}
	ck.SetHTTPOnly(v)
	return nil
}

func (c *cookieBinder) setSameSite(ck *fasthttp.Cookie, tagExpr *tagexpr.TagExpr) error {
	v := getInt(tagExpr, c.sameSiteFS)
	if v == 0 && c.sameSiteRequired {
		return fmt.Errorf("missing required header parameter: %s", c.sameSiteFS)
	}
	ck.SetSameSite(fasthttp.CookieSameSite(v))
	return nil
}

func getInt(tagExpr *tagexpr.TagExpr, fs string) int {
	v := getElem(tagExpr, fs)
	if !v.IsValid() {
		return 0
	}
	return int(v.Int())
}

func getBool(tagExpr *tagexpr.TagExpr, fs string) (ret, valid bool) {
	v := getElem(tagExpr, fs)
	if !v.IsValid() {
		return false, false
	}
	return v.Bool(), true
}

func getTime(tagExpr *tagexpr.TagExpr, fs string) (time.Time, error) {
	v := getElem(tagExpr, fs)
	if !v.IsValid() {
		return time.Time{}, nil
	}
	if v.Kind() == reflect.String {
		return time.Parse(time.RFC1123, v.String())
	}
	if v.CanInterface() {
		t, ok := v.Interface().(time.Time)
		if ok {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("type %s cannot be converted to time.Time", v.Type().String())
}

func getString(tagExpr *tagexpr.TagExpr, fs string) string {
	v := getElem(tagExpr, fs)
	if !v.IsValid() {
		return ""
	}
	return v.String()
}

func getElem(tagExpr *tagexpr.TagExpr, fs string) reflect.Value {
	if fs == "" {
		return reflect.Value{}
	}
	fh, _ := tagExpr.Field(fs)
	v := fh.Value(false)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v
}

func checkString(fieldName string, t reflect.Type) error {
	t = goutil.DereferenceType(t)
	if t.Kind() != reflect.String {
		return fmt.Errorf("header field %s must be string or *string, but got: %s", fieldName, t.String())
	}
	return nil
}

func checkBool(fieldName string, t reflect.Type) error {
	t = goutil.DereferenceType(t)
	if t.Kind() != reflect.Bool {
		return fmt.Errorf("header field %s must be bool or *bool, but got: %s", fieldName, t.String())
	}
	return nil
}

func checkTime(fieldName string, t reflect.Type) error {
	t = goutil.DereferenceType(t)
	if t.String() != "time.Time" {
		return fmt.Errorf("header field %s must be time.Time or *time.Time, but got: %s", fieldName, t.String())
	}
	return nil
}

func checkInt(fieldName string, t reflect.Type) error {
	t = goutil.DereferenceType(t)
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
	default:
		return fmt.Errorf("header field %s must be signed integer or signed integer pointer, but got: %s", fieldName, t.String())
	}
	return nil
}
