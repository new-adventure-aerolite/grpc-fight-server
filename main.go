package main

import (
	"flag"
	"log"
	"net"

	_ "github.com/lib/pq"
	"github.com/new-adventure-aerolite/grpc-fight-server/pd/fight"
	"github.com/new-adventure-aerolite/grpc-fight-server/pkg/connection"
	"github.com/new-adventure-aerolite/grpc-fight-server/pkg/jaeger_service"
	"github.com/new-adventure-aerolite/grpc-fight-server/pkg/service"
	"google.golang.org/grpc"
	"k8s.io/klog"
)

var port string
var config string

func init() {
	flag.StringVar(&port, "port", "8001", "listen port")
	flag.StringVar(&config, "config", "./config/config.json", "connection config file to postgresql")
}

func main() {
	flag.Parse()

	// init tracer
	var servOpts []grpc.ServerOption
	// new jaeger tracer
	tracer, _, err := jaeger_service.NewJaegerTracer("fight-server-backend", "jaeger-collector.istio-system.svc.cluster.local:14268")
	if err != nil {
		klog.Fatal(err)
	}

	servOpts = append(servOpts, jaeger_service.ServerOption(tracer))

	// hold the connection to the pg
	db, listener, err := connection.Create(config)
	if err != nil {
		klog.Fatal(err)
	}
	defer db.Close()

	svc := service.New(db, listener, tracer)

	server := grpc.NewServer(servOpts...)

	fight.RegisterFightSvcServer(server, svc)

	lis, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		log.Fatalf("net.Listen err: %v", err)
	}

	server.Serve(lis)
}
