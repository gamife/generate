package temps

var allService = `package services

import (
)

type IService interface {
    /*g:k1*/
	I{{.Topic}}Service

	HealthCheck() bool
}

type Service struct {
    /*g:k2*/
	{{.Topic}}Service
}

type ServiceMiddleware func(IService) IService`
