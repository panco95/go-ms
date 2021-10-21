package main

import (
	"github.com/gin-gonic/gin"
	"github.com/panco95/go-garden/core"
)

var service *core.Garden

type Rpc struct {
	core.Rpc
}

func main() {
	service = core.New()
	service.Run(service.GatewayRoute, new(Rpc), auth)
}

// Customize the auth middleware
func auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// before logic
		c.Next()
		// after logic
	}
}
