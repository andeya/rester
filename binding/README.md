# binding [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/henrylee2cn/rester/binding)

A powerful fasthttp request parameters binder that supports struct tag expression.

## Syntax

The parameter position in HTTP request:

|expression|renameable|description|
|----------|----------|-----------|
|`path:"$name"` or `path:"$name,required"`|Yes|URL path parameter|
|`query:"$name"` or `query:"$name,required"`|Yes|URL query parameter|
|`raw_body:""` or `raw_body:"required"`|Yes|The raw bytes of body|
|`form:"$name"` or `form:"$name,required"`|Yes|The field in body, support:<br>`application/x-www-form-urlencoded`,<br>`multipart/form-data`|
|`protobuf:"...(raw syntax)"`|No|The field in body, support:<br>`application/x-protobuf`|
|`json:"$name"` or `json:"$name,required"`|No|The field in body, support:<br>`application/json`|
|`header:"$name"` or `header:"$name,required"`|Yes|Header parameter|
|`cookie:"$name"` or `cookie:"$name,required"`|Yes|Cookie parameter|
|`default:"$value"`|Yes|Default parameter|
|`vd:"...(tagexpr validator syntax)"`|Yes|The tagexpr expression of validator|

**NOTE:**

- `"$name"` is variable placeholder
- If `"$name"` is empty, use the name of field
- If `"$name"` is `-`, omit the field
- Expression `required` or `req` indicates that the parameter is required
- `default:"$value"` defines the default value for fallback when no binding is successful
- If no position is tagged, try bind parameters from the body when the request has body,
<br>otherwise try bind from the URL query
- When there are multiple tags or no tags, the order in which to try to bind is:
  1. path
  2. form
  3. query
  4. cookie
  5. header
  6. protobuf
  7. json
  8. default

## Type Unmarshalor

TimeRFC3339-binding function is registered by default.

Register your own binding function for the specified type, e.g.:

```go
MustRegTypeUnmarshal(reflect.TypeOf(time.Time{}), func(v string, emptyAsZero bool) (reflect.Value, error) {
	if v == "" && emptyAsZero {
		return reflect.ValueOf(time.Time{}), nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return reflect.Value{}, err
	}
	return reflect.ValueOf(t), nil
})
```
