package sdkgolib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	_ "github.com/go-chassis/go-chassis/v2/bootstrap"
	"github.com/go-chassis/go-chassis/v2/client/rest"
	"github.com/go-chassis/go-chassis/v2/core"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/suifengpiao14/kvstruct"
	"github.com/suifengpiao14/lineschema/application/validatestream"
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
	SetContext(ctx context.Context)
	GetContext() (ctx context.Context)
	GetStream() (stream stream.StreamInterface)
	ParseResponse(responseBody []byte) (err error)
}

type DefaultImplementPartClientFuncs struct {
	ctx context.Context
}

func (e *DefaultImplementPartClientFuncs) Init() {
}

func (e *DefaultImplementPartClientFuncs) SetContext(ctx context.Context) {

	e.ctx = ctx
}
func (e *DefaultImplementPartClientFuncs) GetContext() (ctx context.Context) {
	if e.ctx == nil {
		e.ctx = context.Background()
	}
	return e.ctx
}

func Run(client ClientInterface) (out []byte, err error) {
	input, err := json.Marshal(client)
	if err != nil {
		return nil, err
	}
	s := client.GetStream()
	out, err = s.Run(client.GetContext(), input)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func DefaultSDKStream(client ClientInterface, lineschemaApi validatestream.LineschemaApi, requestFn RequestFn, errHandlerFn stream.HandlerErrorFn) (s *stream.Stream, err error) {

	in, out, err := validatestream.GetApiStreamHandlerFn(lineschemaApi)
	if err != nil {
		return nil, err
	}
	handlerFns := make([]stream.HandlerFn, 0)
	handlerFns = append(handlerFns, out...)
	handlerFns = append(handlerFns, RequestStreamHandlerFn(client, requestFn))
	handlerFns = append(handlerFns, in...)
	handlerFns = append(handlerFns, ResponseStreamHandlerFn(client))
	s = stream.NewStream(
		errHandlerFn,
		handlerFns...,
	)
	return s, err
}

func ResponseStreamHandlerFn(client ClientInterface) (handlerFn stream.HandlerFn) {
	return func(ctx context.Context, input []byte) (out []byte, err error) {
		err = client.ParseResponse(input)
		return nil, err
	}
}

func RequestStreamHandlerFn(client ClientInterface, requestFn RequestFn) (handlerFn stream.HandlerFn) {
	method, path := client.GetRoute()
	return func(ctx context.Context, input []byte) (out []byte, err error) {
		return requestFn(ctx, method, path, input)
	}
}

type RequestFn func(ctx context.Context, method string, path string, body []byte) (out []byte, err error)

type ContextKey string

const (
	contentKey_contentType ContextKey = "content-type"
)

func SetContentType(client ClientInterface, contentType string) {
	ctx := client.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, contentKey_contentType, contentType)
	client.SetContext(ctx)
}

func GetContentType(client ClientInterface) (contentType string) {
	ctx := client.GetContext()
	contentType = getContentType(ctx)
	return contentType

}

func getContentType(ctx context.Context) (contentType string) {
	v := ctx.Value(contentKey_contentType)
	contentType = cast.ToString(v)
	if contentType == "" {
		contentType = "application/json"
	}
	return contentType
}

// RestyRequestFn 通用请求方法
func RestyRequestFn(host string) (requestFn RequestFn) {
	return func(ctx context.Context, method string, path string, body []byte) (out []byte, err error) {
		r := resty.New().R()
		urlstr := fmt.Sprintf("%s%s", host, path)

		headContentType := "Content-Type"
		if r.Header.Get(headContentType) == "" {
			r.Header.Add(headContentType, getContentType(ctx))
		}

		switch strings.ToUpper(method) {
		case http.MethodGet:
			m, err := str2FormMap(string(body))
			if err != nil {
				return nil, err
			}
			r = r.SetQueryParams(m)
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			r = r.SetBody(body)
		}

		logInfo := &tormcurl.LogInfoHttp{
			GetRequest: func() *http.Request { return r.RawRequest },
		}
		defer func() {
			logchan.SendLogInfo(logInfo)
		}()
		res, err := r.Execute(method, urlstr)
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

}

func ChasissRequestFn(host string) (requestFn RequestFn) {
	return func(ctx context.Context, method string, path string, body []byte) (out []byte, err error) {
		urlstr := fmt.Sprintf("%s%s", host, path)
		r, err := rest.NewRequest(method, urlstr, body)
		if err != nil {
			return nil, err
		}
		res, err := core.NewRestInvoker().ContextDo(ctx, r)
		if err != nil {
			return nil, err
		}
		logInfo := &tormcurl.LogInfoHttp{
			GetRequest: func() *http.Request { return r },
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
}

// str2FormMap 结构体转map[string]string 用于请求参数传递
func str2FormMap(s string) (out map[string]string, err error) {
	strJson, err := kvstruct.FormatValue2String(s, "")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(strJson), &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func JsonMarshal(o interface{}) (out string, err error) {
	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	out = string(b)
	return out, nil
}
