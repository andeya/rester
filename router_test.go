package rester

import (
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
	r.Control("/", &Ctl2{})
	defer func() {
		assert.Equal(t, "a handle is already registered for path '/'", recover())
	}()
	r.Control("/", &Ctl2{})
}

func TestRouter_ControlFrom(t *testing.T) {
	var r Router
	r.ControlFrom("/", func() Controller {
		return &Ctl2{flag: 1}
	})
}
