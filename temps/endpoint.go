package temps

var endpoint = `package endpoints

/*g:p1*/
import (
    "context"
    "$$$/internal/proto"
    "$$$/internal/services"
    "github.com/go-kit/kit/endpoint"
)

/*g:k1*/
func Make{{.FuncName}}Endpoint(svc services.I{{.Topic}}Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(proto.{{.FuncName}}Req)
		data, err := svc.{{.FuncName}}(ctx,req)
		if err != nil {
			return proto.BaseResponse{
				RespCode: 500,
				RespMsg:  err.Error(),
			}, err
		}
		return proto.BaseResponse{
			RespCode: 200,
			RespMsg:  "成功",
			Data:     data,
		}, err
	}
}`
