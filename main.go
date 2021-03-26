package main

import (
	"flag"
	"log"
	"net"

	"github.com/TianqiuHuang/grpc-fight-app/pd/fight"
	"github.com/TianqiuHuang/grpc-fight-app/pkg/connection"
	"github.com/TianqiuHuang/grpc-fight-app/pkg/service"
	_ "github.com/lib/pq"
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

	// hold the connection to the pg
	db, listener, err := connection.Create(config)
	if err != nil {
		klog.Fatal(err)
	}
	defer db.Close()

	svc := service.New(db, listener)

	server := grpc.NewServer()
	fight.RegisterFightSvcServer(server, svc)

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("net.Listen err: %v", err)
	}

	server.Serve(lis)
}
