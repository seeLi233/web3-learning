package resp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int32       `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "success",
		Data: data,
	})
}

func Error(c *gin.Context, code int32, msg string) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
	})
}
