package main

import "github.com/henrylee2cn/rester"

type CorsCtl struct {
	rester.BaseController
}

func (ctl *CorsCtl) CORS_GET() {
	ctl.Logger().Printf("CorsCtl: ============")
	ctl.OK("test CORS")
}

func main() {
	engine := rester.New()
	engine.Control("/", new(CorsCtl))
	err := engine.ListenAndServe(":8080")
	if err != nil {
		panic(err)
	}
	// request:
	//  GET http://localhost:8080/
	// log:
	//  - GET http://localhost:8080/ - CorsCtl: ============
	// response:
	//  "test CORS"
}
