package goms

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// GinServer 开启Gin服务
// @param port 监听端口
// @param serviceName 服务名称
// @param route gin路由
// @param auth 鉴权中间件
func GinServer(port string, route func(r *gin.Engine), auth func() gin.HandlerFunc) error {
	gin.SetMode("release")
	server := gin.Default()
	path, _ := os.Getwd()
	err := CreateDir(path + "/runtime")
	if err != nil {
		return errors.New("[Create runtime folder] " + err.Error())
	}
	file, err := os.Create(fmt.Sprintf("%s/runtime/gin_%s.log", path, ServiceName))
	if err != nil {
		return errors.New("[Create gin log file] " + err.Error())
	}
	gin.DefaultWriter = file
	server.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage)
	}))
	server.Use(gin.Recovery())
	server.Use(Trace())
	if auth != nil {
		server.Use(auth())
	}
	route(server)

	log.Printf("[%s] Http Listen on port: %s", ServiceName, port)
	return server.Run(":" + port)
}

// GatewayRoute 网关路由解析
// 第一个参数：下游服务名称
// 第二个参数：下游服务接口路由
func GatewayRoute(r *gin.Engine) {
	r.Any("api/:service/:action", func(c *gin.Context) {
		// 服务名称和服务路由
		service := c.Param("service")
		action := c.Param("action")
		// 从reqTrace获取相关请求报文
		traceLog, err := GetTraceLog(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, FailRes())
			Logger.Error(err)
			return
		}
		method := traceLog.Request.Method
		headers := traceLog.Request.Headers
		urlParam := traceLog.Request.UrlParam
		body := traceLog.Request.Body
		requestId := traceLog.RequestId

		// 请求下游服务
		data, err := CallService(service, action, method, urlParam, body, headers, requestId)
		if err != nil {
			Logger.Error("call " + service + "/" + action + " error: " + err.Error())
			c.JSON(http.StatusInternalServerError, FailRes())
			return
		}
		var result Any
		err = json.Unmarshal([]byte(data), &result)
		if err != nil {
			Logger.Error(service + "/" + action + " return invalid format: " + data)
			c.JSON(http.StatusInternalServerError, FailRes())
			return
		}
		c.JSON(http.StatusOK, SuccessRes(result))
	})

	// 集群信息查询接口
	r.Any("cluster", func(c *gin.Context) {
		c.JSON(http.StatusOK, SuccessRes(Any{
			"services": Services,
		}))
	})
}

// Trace 链路追踪调试中间件
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 生成唯一requestId，提供给下游服务获取
		requestId := c.GetHeader("X-Request-Id")
		startEvent := "service.start"
		endEvent := "service.end"
		if requestId == "" || false == ParseUuid(requestId) {
			requestId = NewUuid()
			startEvent = "request.start"
			endEvent = "request.end"
		}

		traceLog := TraceLog{
			ProjectName: ProjectName,
			ServiceName: ServiceName,
			ServiceId:   ServiceId,
			RequestId:   requestId,
			Request: Request{
				ClientIp: GetClientIp(c),
				Method:   GetMethod(c),
				UrlParam: GetUrlParam(c),
				Headers:  GetHeaders(c),
				Body:     GetBody(c),
				Url:      GetUrl(c),
			},
			Event: startEvent,
			Time:  ToDatetime(start),
		}

		// 记录远程调试日志
		PushTraceLog(&traceLog)
		// 封装到gin请求上下文
		c.Set("traceLog", &traceLog)

		// 执行请求接口
		c.Next()
		c.Abort()

		// 接口执行完毕后执行
		// 记录远程调试日志，代表当前请求完毕
		end := time.Now()
		timing := Timing(start, end)
		traceLog.Event = endEvent
		traceLog.Time = ToDatetime(end)
		traceLog.Trace = Any{
			"timing": timing,
		}
		PushTraceLog(&traceLog)
	}
}

// CheckCallServiceKey 服务调用安全验证
func CheckCallServiceKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestKey := c.GetHeader("Call-Service-Key")
		if strings.Compare(requestKey, viper.GetString("callServiceKey")) != 0 {
			c.JSON(http.StatusForbidden, FailRes())
			c.Abort()
		}
	}
}

// GetTraceLog 获取reqTrace上下文
func GetTraceLog(c *gin.Context) (*TraceLog, error) {
	t, success := c.Get("traceLog")
	if !success {
		return nil, errors.New("traceLog is nil")
	}
	tl := t.(*TraceLog)
	return tl, nil
}

// GetMethod 获取请求方式
func GetMethod(c *gin.Context) string {
	return strings.ToUpper(c.Request.Method)
}

// GetClientIp 获取请求客户端ip
func GetClientIp(c *gin.Context) string {
	return c.ClientIP()
}

// GetBody 获取请求body
func GetBody(c *gin.Context) Any {
	body := Any{}
	h := c.GetHeader("Content-Type")
	// 获取表单格式请求参数
	if strings.Contains(h, "multipart/form-data") || strings.Contains(h, "application/x-www-form-urlencoded") {
		c.PostForm("get_params_bug_fix")
		for k, v := range c.Request.PostForm {
			body[k] = v[0]
		}
		// 获取json格式请求参数
	} else if strings.Contains(h, "application/json") {
		c.BindJSON(&body)
	}
	return body
}

// GetUrl 获取请求路径
func GetUrl(c *gin.Context) string {
	return c.Request.URL.Path
}

// GetUrlParam 获取请求query参数
func GetUrlParam(c *gin.Context) string {
	requestUrl := c.Request.RequestURI
	urlSplit := strings.Split(requestUrl, "?")
	if len(urlSplit) > 1 {
		requestUrl = "?" + urlSplit[1]
	} else {
		requestUrl = ""
	}
	return requestUrl
}

// GetHeaders 获取请求头map
func GetHeaders(c *gin.Context) Any {
	headers := Any{}
	for k, v := range c.Request.Header {
		headers[k] = v[0]
	}
	return headers
}
