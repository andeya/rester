package main

import "github.com/henrylee2cn/rester"

func main() {
	engine := rester.New()
	engine.ServeFiles("/assets/*filepath", "../../")
	err := engine.ListenAndServe(":8080")
	if err != nil {
		panic(err)
	}
	// request:
	//  GET http://localhost:8080/assets/README.md
}
