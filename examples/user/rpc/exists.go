package rpc

import (
	"context"
	"github.com/panco95/go-garden/examples/user/global"
	"github.com/panco95/go-garden/examples/user/rpc/define"
)

func (r *Rpc) Exists(ctx context.Context, args *define.ExistsArgs, reply *define.ExistsReply) error {
	span := global.Service.StartRpcTrace(ctx, args, "Exists")

	reply.Exists = false
	if _, ok := global.Users.Load(args.Username); ok {
		reply.Exists = true
	}

	global.Service.FinishRpcTrace(span)
	return nil
}
