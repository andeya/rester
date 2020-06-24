// Copyright 2020 HenryLee. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
