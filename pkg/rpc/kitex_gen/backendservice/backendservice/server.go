// Code generated by Kitex v0.8.0. DO NOT EDIT.
package backendservice

import (
	server "github.com/cloudwego/kitex/server"
	backendservice "github.com/selectdb/ccr_syncer/pkg/rpc/kitex_gen/backendservice"
)

// NewServer creates a server.Server with the given handler and options.
func NewServer(handler backendservice.BackendService, opts ...server.Option) server.Server {
	var options []server.Option

	options = append(options, opts...)

	svr := server.NewServer(options...)
	if err := svr.RegisterService(serviceInfo(), handler); err != nil {
		panic(err)
	}
	return svr
}
