package temps

var allEndpoint = `package endpoints

func MakeAllEndpoint(svc services.IService, tracer *zipkin.Tracer) (eps AllEndpoint) {

    /*g:k1*/
	{{.FuncNameSmall}}Endpoint := Make{{.FuncName}}Endpoint(svc)
    /*g:k2*/
	{{.FuncNameSmall}}Endpoint = kitzipkin.TraceEndpoint(tracer, "{{.FuncNameSnack}}")({{.FuncNameSmall}}Endpoint)

	eps = AllEndpoint{
	    /*g:k3*/
		{{.FuncName}}Endpoint:    {{.FuncNameSmall}}Endpoint,
	}
	return
}


type AllEndpoint struct {
    /*g:k4*/
	{{.FuncName}}Endpoint endpoint.Endpoint
}
`
