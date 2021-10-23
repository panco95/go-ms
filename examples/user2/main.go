package main

import (
	"github.com/panco95/go-garden/core"
	"github.com/panco95/go-garden/examples/user2/api"
	"github.com/panco95/go-garden/examples/user2/global"
	"github.com/panco95/go-garden/examples/user2/rpc"
)

func main() {
	global.Service = core.New()
	global.Service.Run(api.Routes, new(rpc.Rpc), nil)
}
