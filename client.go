package sdkgolib

import (
	"context"
	"io"
	"net/http"

	_ "github.com/go-chassis/go-chassis/v2/bootstrap"
	"github.com/go-chassis/go-chassis/v2/core"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/torm"
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
	Request(ctx context.Context) (err error)
	GetDescription() (title string, description string)
	GetName() (domain string, name string)
	GetOutRef() (outRef error)
	RequestHandler(ctx context.Context, input []byte) (out []byte, err error)
	GetSDKConfig() (sdkConfig Config)
}

type DefaultImplementPartClientFuncs struct {
}

func (e *DefaultImplementPartClientFuncs) Init() {
}

// RequestFn 封装http请求数据格式
type RequestFn func(ctx context.Context, req *http.Request) (out []byte, err error)

// RestyRequestFn 通用请求方法
func RestyRequestFn(ctx context.Context, req *http.Request) (out []byte, err error) {
	r := resty.New().R()
	urlstr := req.URL.String()
	r.Header = req.Header
	r.FormData = req.Form
	r.RawRequest = req
	if req.Body != nil {
		var body io.ReadCloser
		body, err = req.GetBody()
		if err == nil && body != nil {
			defer body.Close()
			b, err := io.ReadAll(body)
			if err != nil {
				return nil, err
			}
			r.SetBody(b)
		}
	}

	logInfo := &torm.LogInfoHttp{
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
	logInfo := &torm.LogInfoHttp{
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
