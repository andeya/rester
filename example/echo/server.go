package main

import "github.com/henrylee2cn/rester"

type MwCtl struct {
	rester.BaseCtl
	skip bool
}

func (ctl *MwCtl) Any(args struct {
	A string `query:"a"`
}) {
	ctl.Logger().Printf("MwCtl: a=%s", args.A)
	if !ctl.skip {
		ctl.SetUserValue("a", args.A)
	}
}

type EchoCtl struct {
	MwCtl
}

func (ctl *EchoCtl) GET(args struct {
	B []string `query:"b"`
}) {
	ctl.Logger().Printf("EchoCtl: b=%v", args.B)
	ctl.OK(rester.H{
		"a": ctl.UserValue("a"),
		"b": args.B,
	})
}

func main() {
	engine := rester.New()
	engine.DefControl("/", new(EchoCtl))
	engine.Control("/from", func() rester.Controller {
		return &EchoCtl{
			MwCtl{skip: true},
		}
	})
	err := engine.ListenAndServe(":8080")
	if err != nil {
		panic(err)
	}
	// request:
	//  GET http://localhost:8080/?a=x&b=y&b=z
	// log:
	//  - MwCtl: a=x
	//  - EchoCtl: b=[y z]
	// response:
	//  {"a":"x","b":["y","z"]}

	// request:
	//  GET http://localhost:8080/from?a=x&b=y&b=z
	// log:
	//  - MwCtl: a=x
	//  - EchoCtl: b=[y z]
	// response:
	//  {"a":null,"b":["y","z"]}
}
