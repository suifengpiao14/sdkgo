package sdkgolib

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	_ "github.com/go-chassis/go-chassis/v2/bootstrap"
	"github.com/go-chassis/go-chassis/v2/core"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/lineschema/application/lineschemapacket"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/stream"
	"github.com/suifengpiao14/torm/tormcurl"
)

var (
	API_NOT_FOUND = errors.Errorf("not found client")
)

type DefaultImplementClientOutput struct{}

func (c DefaultImplementClientOutput) Error() (err error) {
	return nil
}

type ClientInterface interface {
	GetRoute() (method string, path string)
	Init()
	GetDescription() (title string, description string)
	GetName() (domain string, name string)
	GetStream() (stream stream.StreamInterface, err error)
	RequestHandler(ctx context.Context, input []byte) (out []byte, err error)
	ResponseHandler(ctx context.Context, responseBody []byte) (out []byte, err error)
}

type DefaultImplementPartClientFuncs struct {
}

func (e *DefaultImplementPartClientFuncs) Init() {
}

func Run(ctx context.Context, client ClientInterface) (err error) {
	input, err := json.Marshal(client)
	if err != nil {
		return err
	}
	s, err := client.GetStream()
	if err != nil {
		return err
	}
	_, err = s.Run(ctx, input)
	if err != nil {
		return err
	}
	return nil
}

func DefaultSDKStream(client ClientInterface, lineschemaPacket lineschemapacket.LineschemaPacketI) (s *stream.Stream, err error) {

	in, out, err := lineschemapacket.GetLineschemaPackageHandlerFn(lineschemaPacket)
	if err != nil {
		return nil, err
	}
	handlerFns := make([]stream.HandlerFn, 0)
	handlerFns = append(handlerFns, out...)
	handlerFns = append(handlerFns, client.RequestHandler)
	handlerFns = append(handlerFns, in...)
	handlerFns = append(handlerFns, client.ResponseHandler)
	s = stream.NewStream(
		nil,
		handlerFns...,
	)
	return s, err
}

//RequestFn 封装http请求数据格式
type RequestFn func(ctx context.Context, req *http.Request) (out []byte, err error)

// RestyRequestFn 通用请求方法
func RestyRequestFn(ctx context.Context, req *http.Request) (out []byte, err error) {
	r := resty.New().R()
	urlstr := req.URL.String()
	r.Header = req.Header
	r.FormData = req.Form
	r.RawRequest = req

	logInfo := &tormcurl.LogInfoHttp{
		GetRequest: func() *http.Request { return r.RawRequest },
	}
	defer func() {
		logchan.SendLogInfo(logInfo)
	}()
	res, err := r.Execute(req.Method, urlstr)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	responseBody := res.Body()
	logInfo.ResponseBody = string(responseBody)
	logInfo.Response = res.RawResponse
	return responseBody, nil

}

func ChasissRequestFn(ctx context.Context, req *http.Request) (out []byte, err error) {
	res, err := core.NewRestInvoker().ContextDo(ctx, req)
	if err != nil {
		return nil, err
	}
	logInfo := &tormcurl.LogInfoHttp{
		GetRequest: func() *http.Request { return req },
	}
	defer func() {
		logchan.SendLogInfo(logInfo)
	}()
	defer res.Body.Close()
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	logInfo.ResponseBody = string(responseBody)
	logInfo.Response = res
	return responseBody, nil

}
