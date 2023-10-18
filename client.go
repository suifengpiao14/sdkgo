package sdkgolib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/gojsonschemavalidator"
	"github.com/suifengpiao14/jsonschemaline"
	"github.com/suifengpiao14/kvstruct"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/torm/tormcurl"
	"github.com/tidwall/gjson"
	"github.com/xeipuuv/gojsonschema"
)

var (
	API_NOT_FOUND = errors.Errorf("not found client")
)

type ClientOutputI interface {
	Error() (err error) // 判断结果是否有错误,没有错误,认为成功
}

type DefaultImplementClientOutput struct{}

func (c DefaultImplementClientOutput) Error() (err error) {
	return nil
}

type ClientInterface interface {
	GetInputSchema() (lineschema string)
	GetOutputSchema() (lineschema string)
	GetRoute() (method string, path string)
	Init()
	GetDescription() (title string, description string)
	GetName() (domain string, name string)
	GetOutputRef() (output ClientOutputI)
	Request(ctx context.Context) (err error)
	GetCClient(c ClientInterface) (cClient *_Client, err error)
}

type DefaultImplementPartClientFuncs struct{}

func (e *DefaultImplementPartClientFuncs) GetInputSchema() (lineschema string) {
	return ""
}
func (e *DefaultImplementPartClientFuncs) GetOutputSchema() (lineschema string) {
	return ""
}

func (e *DefaultImplementPartClientFuncs) Init() {
}

func (e *DefaultImplementPartClientFuncs) GetCClient(c ClientInterface) (cClient *_Client, err error) {
	cClient, err = GetClient(c)
	if err != nil {
		return nil, err
	}
	return cClient, nil
}

type LogName string

func (logName LogName) String() (name string) {
	return string(logName)
}

const (
	LOG_INFO_EXEC_Client_HANDLER LogName = "LogInfoExecClientHandler"
)

type LogInfoClientRun struct {
	Input          string
	DefaultJson    string
	MergedDefault  string
	Err            error `json:"error"`
	FormattedInput string
	OriginalOut    string
	Out            string
	logchan.EmptyLogInfo
}

func (l *LogInfoClientRun) GetName() logchan.LogName {
	return LOG_INFO_EXEC_Client_HANDLER
}
func (l *LogInfoClientRun) Error() error {
	return l.Err
}

type _Client struct {
	ClientInterface
	inputFormatGjsonPath  string
	defaultJson           string
	outputFormatGjsonPath string
	validateInputLoader   gojsonschema.JSONLoader
	validateOutputLoader  gojsonschema.JSONLoader
}

var clientMap sync.Map

const (
	clientMap_route_add_key = "___all_client_add___"
)

// RegisterClient 创建处理器，内部逻辑在接收请求前已经确定，后续不变，所以有错误直接panic ，能正常启动后，这部分不会出现错误
func RegisterClient(ClientInterface ClientInterface) (err error) {
	method, path := ClientInterface.GetRoute()
	key := getRouteKey(method, path)
	// 以下初始化可以复用,线程安全
	api := &_Client{
		ClientInterface: ClientInterface,
	}
	inputSchema := ClientInterface.GetInputSchema()
	if inputSchema != "" {
		api.validateInputLoader, err = newJsonschemaLoader(inputSchema)
		if err != nil {
			return err
		}
		inputLineSchema, err := jsonschemaline.ParseJsonschemaline(inputSchema)
		if err != nil {
			return err
		}
		api.inputFormatGjsonPath = inputLineSchema.GjsonPath(true, jsonschemaline.FormatPathFnByFormatOut) // 这个地方要反向，将输入的字符全部转为字符串，供网络传输
		defaultInputJson, err := inputLineSchema.DefaultJson()
		if err != nil {
			err = errors.WithMessage(err, "get input default json error")
			return err
		}
		api.defaultJson = defaultInputJson.Json
	}
	outputSchema := ClientInterface.GetOutputSchema()
	if outputSchema != "" {
		api.validateOutputLoader, err = newJsonschemaLoader(outputSchema)
		if err != nil {
			return err
		}
		outputLineSchema, err := jsonschemaline.ParseJsonschemaline(outputSchema)
		if err != nil {
			return err
		}
		api.outputFormatGjsonPath = outputLineSchema.GjsonPath(true, jsonschemaline.FormatPathFnByFormatIn) // 这个地方要反向，将输入的字符全部转为结构体类型，供程序应用
	}
	clientMap.Store(key, api)
	routes := make(map[string][2]string, 0)
	if routesI, ok := clientMap.Load(clientMap_route_add_key); ok {
		if old, ok := routesI.(map[string][2]string); ok {
			routes = old
		}
	}
	route := [2]string{method, path}
	routes[key] = route
	clientMap.Store(clientMap_route_add_key, routes)
	return nil
}

func GetClient(client ClientInterface) (cClient *_Client, err error) {
	method, path := client.GetRoute()
	key := getRouteKey(method, path)
	apiAny, ok := clientMap.Load(key)
	if !ok {
		//延迟注册
		rt := reflect.TypeOf(client).Elem()
		rv := reflect.New(rt)
		_client := rv.Interface().(ClientInterface)
		_client.Init()
		err = RegisterClient(_client)
		if err != nil {
			return nil, err
		}
		apiAny, ok = clientMap.Load(key)
		if !ok {
			return cClient, errors.WithMessagef(API_NOT_FOUND, "method:%s,path:%s", method, path)
		}
	}

	exitsApi := apiAny.(*_Client)
	client.Init()
	cClient = &_Client{
		ClientInterface:       client,
		validateInputLoader:   exitsApi.validateInputLoader,
		validateOutputLoader:  exitsApi.validateOutputLoader,
		inputFormatGjsonPath:  exitsApi.inputFormatGjsonPath,
		outputFormatGjsonPath: exitsApi.outputFormatGjsonPath,
		defaultJson:           exitsApi.defaultJson,
	}
	return cClient, nil
}

func (a _Client) inputValidate(input string) (err error) {
	if a.validateInputLoader == nil {
		return nil
	}
	inputStr := string(input)
	err = gojsonschemavalidator.Validate(inputStr, a.validateInputLoader)
	if err != nil {
		return err
	}
	return nil
}
func (a _Client) outputValidate(output string) (err error) {
	outputStr := string(output)
	if a.validateOutputLoader == nil {
		return nil
	}
	err = gojsonschemavalidator.Validate(outputStr, a.validateOutputLoader)
	if err != nil {
		return err
	}
	return nil
}

func (a _Client) modifyTypeByFormat(input string, formatGjsonPath string) (formattedInput string, err error) {
	formattedInput = input
	if formatGjsonPath == "" {
		return formattedInput, nil
	}
	formattedInput = gjson.Get(input, formatGjsonPath).String()
	return formattedInput, nil
}

func (a _Client) convertOutput(out string) (err error) {
	err = json.Unmarshal([]byte(out), a.ClientInterface.GetOutputRef())
	if err != nil {
		return err
	}
	return nil
}

//FormatAsIntput 供外部格式化输出
func (a _Client) FormatAsIntput(input string) (formatedInput string, err error) {
	formatedInput, err = a.modifyTypeByFormat(input, a.inputFormatGjsonPath)
	return formatedInput, err
}

//FormatAsOutput 供外部格式化输出
func (a _Client) FormatAsOutput(output string) (formatedOutput string, err error) {
	formatedOutput, err = a.modifyTypeByFormat(output, a.outputFormatGjsonPath)
	return formatedOutput, err
}

func (a _Client) Request() {

}

// RequestFn 通用请求方法
func (a _Client) RequestFn(ctx context.Context, host string) (err error) {
	b, err := json.Marshal(a.ClientInterface)
	if err != nil {
		return err
	}
	inputStr := string(b)
	// 合并默认值
	if a.defaultJson != "" {
		inputStr, err = jsonschemaline.MergeDefault(inputStr, a.defaultJson)
		if err != nil {
			err = errors.WithMessage(err, "merge default value error")
			return err
		}
	}
	err = a.inputValidate(inputStr)
	if err != nil {
		return err
	}
	//将format 中 int,float,bool 应用到数据
	formattedInput, err := a.FormatAsIntput(inputStr)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	err = json.Unmarshal([]byte(formattedInput), &params)
	if err != nil {
		return err
	}
	outByte, err := RequestFn(ctx, a.ClientInterface, host)
	if err != nil {
		return err
	}
	originalOut := string(outByte)
	err = a.outputValidate(originalOut) // 先验证网络数据
	if err != nil {
		return err
	}
	outStr, err := a.FormatAsOutput(originalOut) // 网络数据ok，内部转换
	if err != nil {
		return err
	}

	err = a.convertOutput(outStr)
	if err != nil {
		return err
	}
	err = a.GetOutputRef().Error()
	if err != nil {
		return err
	}

	return nil
}

// RequestFn 通用请求方法
func RequestFn(ctx context.Context, input ClientInterface, host string) (out []byte, err error) {
	method, path := input.GetRoute()
	urlstr := fmt.Sprintf("%s%s", host, path)
	r := resty.New().NewRequest()
	params, err := Struct2FormMap(input)
	if err != nil {
		return nil, err
	}
	switch strings.ToUpper(method) {
	case http.MethodGet:
		r = r.SetQueryParams(params)
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		r = r.SetBody(params)
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

// Struct2FormMap 结构体转map[string]string 用于请求参数传递
func Struct2FormMap(v any) (out map[string]string, err error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	strJson, err := kvstruct.FormatValue2String(string(b), "")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(strJson), &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func getRouteKey(method string, path string) (key string) {
	return fmt.Sprintf("%s_%s", strings.ToLower(method), path)
}

func newJsonschemaLoader(lineSchemaStr string) (jsonschemaLoader gojsonschema.JSONLoader, err error) {
	if lineSchemaStr == "" {
		err = errors.Errorf("NewJsonschemaLoader: arg lineSchemaStr required,got empty")
		return nil, err
	}
	inputlineSchema, err := jsonschemaline.ParseJsonschemaline(lineSchemaStr)
	if err != nil {
		return nil, err
	}
	jsb, err := inputlineSchema.JsonSchema()
	if err != nil {
		return nil, err
	}
	jsonschemaStr := string(jsb)
	jsonschemaLoader = gojsonschema.NewStringLoader(jsonschemaStr)
	return jsonschemaLoader, nil
}

func JsonMarshal(o interface{}) (out string, err error) {
	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	out = string(b)
	return out, nil
}
