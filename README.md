# Rester [![report card](https://goreportcard.com/badge/github.com/henrylee2cn/rester?style=flat-square)](http://goreportcard.com/report/henrylee2cn/rester) [![GitHub release](https://img.shields.io/github/release/henrylee2cn/rester.svg?style=flat-square)](https://github.com/henrylee2cn/rester/releases) [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://pkg.go.dev/github.com/henrylee2cn/rester?tab=doc) [![view Go网络编程群](https://img.shields.io/badge/官方QQ群-Go网络编程(42730308)-27a5ea.svg?style=flat-square)](http://jq.qq.com/?_wv=1027&k=fzi4p1)

Fast and concise RESTful web framework based on [fasthttp](https://github.com/valyala/fasthttp).

## Example

```go
package main

import "github.com/henrylee2cn/rester"

type Echo1Ctl struct {
	rester.BaseCtl
	skip bool
}

func (ctl *Echo1Ctl) Any(args struct {
	A string `query:"a"`
}) {
	ctl.Logger().Printf("Echo1Ctl: a=%s", args.A)
	if !ctl.skip {
		ctl.SetUserValue("a", args.A)
	}
}

type Echo2Ctl struct {
	Echo1Ctl
}

func (ctl *Echo2Ctl) GET(args struct {
	B []string `query:"b"`
}) {
	ctl.Logger().Printf("Echo2Ctl: b=%v", args.B)
	ctl.OK(rester.H{
		"a": ctl.UserValue("a"),
		"b": args.B,
	})
}

func main() {
	engine := rester.New()
	engine.Control("/", new(Echo2Ctl))
	engine.ControlFrom("/from", func() rester.Controller {
		return &Echo2Ctl{
			Echo1Ctl{skip: true},
		}
	})
	err := engine.ListenAndServe(":8080")
	if err != nil {
		panic(err)
	}
	// request:
	//  GET http://localhost:8080/?a=x&b=y&b=z
	// log:
	//  - Echo1Ctl: a=x
	//  - Echo2Ctl: b=[y z]
	// response:
	//  {"a":"x","b":["y","z"]}

	// request:
	//  GET http://localhost:8080/from?a=x&b=y&b=z
	// log:
	//  - Echo1Ctl: a=x
	//  - Echo2Ctl: b=[y z]
	// response:
	//  {"a":null,"b":["y","z"]}
}
```

[More examples](https://github.com/henrylee2cn/rester/tree/master/example)

## Binding

[binding doc](https://github.com/henrylee2cn/rester/blob/master/binding/README.md)

## Controller

Design of method call chain for anonymous field of controller

![Struct Method Chain](https://github.com/henrylee2cn/rester/raw/master/doc/chain.png)
