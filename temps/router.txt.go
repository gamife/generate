package temps

var router = `package {{.Package}}

import (
	"context"
	"$$$/internal/endpoints"
	"$$$/internal/proto"
	"$$$/internal/transports/base"
	"git.ningdatech.com/ningda/gin_valid/gin/b"
	"github.com/gin-gonic/gin"
	kitjwt "github.com/go-kit/kit/auth/jwt"
	kithttp "github.com/go-kit/kit/transport/http"
	"net/http"
)

/*g:k1*/
func Load{{.TopicBig}}(e *gin.RouterGroup, endpoint endpoints.AllEndpoint, options ...kithttp.ServerOption) {
        /*g:k2*/
        e.POST("{{.Url}}", base.ConvertGinHandlerFunc(kithttp.NewServer(
		endpoint.{{.FuncName}}Endpoint,
		Decode{{.FuncName}}Request,
		base.EncodeResponse,
		append(options, kithttp.ServerBefore(kitjwt.HTTPToContext()))...,
	)))
}

/*g:k3*/
func Decode{{.FuncName}}Request(_ context.Context, r *http.Request) (interface{}, error) {
	var req proto.{{.FuncName}}Req
	err := b.ShouldBindJSON(r, &req)
	if err != nil {
		return nil, err
	}
	return req, nil
}`
