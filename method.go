package rester

import (
	"strings"

	"github.com/henrylee2cn/ameda"
)

const anyMethod = "Any"

var httpMethodList = []string{
	"GET", "POST", "PUT", "PATCH", "HEAD", "OPTIONS", "DELETE", "CONNECT", "TRACE",
	"CORS_GET", "CORS_POST", "CORS_PUT", "CORS_PATCH", "CORS_HEAD", "CORS_DELETE", "CORS_TRACE",
}

func splitMethod(ctlMethod string) (httpMethod string, cors bool) {
	if !ameda.StringsIncludes(httpMethodList, ctlMethod) {
		return
	}
	httpMethod = strings.TrimPrefix(ctlMethod, "CORS_")
	cors = httpMethod != ctlMethod
	return
}
