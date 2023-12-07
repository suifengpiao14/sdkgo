package gochassis

import (
	"context"
	"io"
	"net/http"

	"github.com/go-chassis/go-chassis/core"
	_ "github.com/go-chassis/go-chassis/v2/bootstrap"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/torm"
)

//ChasissRequestFn 使用g chassis 通信，应为启用这个会初始化注册服务，所以单独一个包，不用是不需要初始哈gochassis
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
