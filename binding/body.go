package binding

import (
	"bytes"
	jsonpkg "encoding/json"
	"errors"

	"github.com/gogo/protobuf/proto"
	"github.com/henrylee2cn/ameda"
	"github.com/valyala/fasthttp"
)

func getBodyInfo(req *fasthttp.RequestCtx) (codec, []byte, error) {
	bodyCodec := getBodyCodec(req)
	bodyBytes, err := getBody(req, bodyCodec)
	return bodyCodec, bodyBytes, err
}

func getBodyCodec(req *fasthttp.RequestCtx) codec {
	ct := req.Request.Header.ContentType()
	idx := bytes.Index(ct, []byte{';'})
	if idx != -1 {
		ct = bytes.TrimRight(ct[:idx], " ")
	}
	switch ameda.UnsafeBytesToString(ct) {
	case "application/json":
		return bodyJSON
	case "application/x-protobuf":
		return bodyProtobuf
	case "application/x-www-form-urlencoded", "multipart/form-data":
		return bodyForm
	default:
		return bodyUnsupport
	}
}

func getBody(req *fasthttp.RequestCtx, bodyCodec codec) ([]byte, error) {
	return req.Request.Body(), nil
}

func bindJSON(pointer interface{}, bodyBytes []byte) error {
	if jsonUnmarshalFunc != nil {
		return jsonUnmarshalFunc(bodyBytes, pointer)
	}
	return jsonpkg.Unmarshal(bodyBytes, pointer)
}

func bindProtobuf(pointer interface{}, bodyBytes []byte) error {
	msg, ok := pointer.(proto.Message)
	if !ok {
		return errors.New("protobuf content type is not supported")
	}
	return proto.Unmarshal(bodyBytes, msg)
}
