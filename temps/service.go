package temps

var service = `package services

/*g:k0*/
import (
	"context"
	"$$$/internal/proto"
)

/*g:k1*/
type I{{.TopicBig}}Service interface {
}

/*g:k2*/
type {{.TopicBig}}Service struct {
}

type IService interface {
    /*g:k3*/
	{{.FuncName}}(context.Context, proto.{{.FuncName}}Req) (*proto.{{.FuncName}}Response, error)
}

/*g:k4*/
func (s *{{.TopicBig}}Service) {{.FuncName}}(context.Context, proto.{{.FuncName}}Req) (*proto.{{.FuncName}}Response, error){
	/*g:k5*/
	return nil, nil
}`
