package api

import (
	"github.com/gin-gonic/gin"
	"github.com/panco95/go-garden/examples/user/global"
)

func Login(c *gin.Context) {
	c.JSON(500,nil)
	return
	var validate struct {
		Username string `form:"username" binding:"required,max=20,min=1"`
	}
	if err := c.ShouldBind(&validate); err != nil {
		Fail(c, MsgInvalidParams)
		return
	}
	username := c.PostForm("username")
	global.Users.Store(username, 1)
	Success(c, MsgOk, nil)
}
