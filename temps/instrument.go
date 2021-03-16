package temps

var instrument = `package services

import (
	"context"
	"$$$/internal/proto"
	"fmt"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/metrics"
	"github.com/juju/ratelimit"
	"golang.org/x/time/rate"
	"time"
)

var ErrLimitExceed = fmt.Errorf("Rate limit exceed!")

// NewTokenBucketLimitterWithJuju 使用juju/ratelimit创建限流中间件
func NewTokenBucketLimitterWithJuju(bkt *ratelimit.Bucket) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			if bkt.TakeAvailable(1) == 0 {
				return nil, ErrLimitExceed
			}
			return next(ctx, request)
		}
	}
}

// NewTokenBucketLimitterWithBuildIn 使用x/time/rate创建限流中间件
func NewTokenBucketLimitterWithBuildIn(bkt *rate.Limiter) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			if !bkt.Allow() {
				return nil, ErrLimitExceed
			}
			return next(ctx, request)
		}
	}
}

// metricMiddleware 定义监控中间件，嵌入Service
// 新增监控指标项：requestCount和requestLatency
type $$$MetricMiddleware struct {
	IService
	requestCount   metrics.Counter
	requestLatency metrics.Histogram
}

// 这里如果是新项目要记得改 main.go里的对应名字
// Metrics 封装监控方法
func $$$Metrics(requestCount metrics.Counter, requestLatency metrics.Histogram) ServiceMiddleware {
	return func(next IService) IService {
		return contractMetricMiddleware{
			next,
			requestCount,
			requestLatency}
	}
}

/*g:k1*/
func (mw $$$MetricMiddleware) {{.FuncName}}(c context.Context, req proto.{{.FuncName}}Req) (*proto.{{.FuncName}}Response, error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "{{.FuncName}}"}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return mw.IService.{{.FuncName}}(c, req)
}
`
