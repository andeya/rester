package rester

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Ctl1 struct {
	BaseCtl
}

func (*Ctl1) Any() {
}

type Ctl2 struct {
	Ctl1
	flag int
}

func (*Ctl2) GET() {
}

func TestRouter_Control(t *testing.T) {
	var r Router
	r.Control("/", func() Controller {
		return &Ctl2{flag: 1}
	})
	// [RESTER] GET     /                         --> github.com/henrylee2cn/rester.Ctl2
}

func TestRouter_DefControl1(t *testing.T) {
	var r Router
	r.DefControl("/", &Ctl2{})
	defer func() {
		assert.Equal(t, "a handle is already registered for path '/'", recover())
	}()
	r.DefControl("/", &Ctl2{})
}

func TestRouter_DefControl2(t *testing.T) {
	var r Router
	type Ctl3 struct {
		Ctl2
	}
	defer func() {
		assert.Equal(t,
			"*rester.Ctl3 has no method with the same name as HTTP method, eg. GET or CORS_GET",
			fmt.Sprint(recover()),
		)
	}()
	r.DefControl("/", &Ctl3{})
}
