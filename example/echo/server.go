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
