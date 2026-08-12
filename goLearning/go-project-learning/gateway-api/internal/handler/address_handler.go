package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/global"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
	"github.com/go-project-learning/project/user-srv/api/pb"
)

func CreateAddress(c *gin.Context) {
	var req struct {
		User_ID   int    `json:"user_id" binding:"required"`
		Receiver  string `json:"receiver" binding:"required"`
		Phone     string `json:"phone" binding:"required"`
		Province  string `json:"province" binding:"required"`
		City      string `json:"city" binding:"required"`
		District  string `json:"district" binding:"required"`
		Detail    string `json:"detail" binding:"required"`
		IsDefault bool   `json:"is_default"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误")
		return
	}

	grpcResp, err := global.AddressClient.CreateAddress(c.Request.Context(), &pb.CreateAddressRequest{
		UserId:    int64(req.User_ID),
		Receiver:  req.Receiver,
		Phone:     req.Phone,
		Province:  req.Province,
		City:      req.City,
		District:  req.District,
		Detail:    req.Detail,
		IsDefault: req.IsDefault,
	})

	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, grpcResp.Data)
}

func DeleteAddress(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, 10001, "id 参数错误")
		return
	}

	_, err = global.AddressClient.DeleteAddress(c.Request.Context(), &pb.DeleteAddressRequest{
		Id: id,
	})

	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	resp.OK(c, nil)
}

func UpdateAddress(c *gin.Context) {
	var req struct {
		Id        int    `json:"id" binding:"required"`
		Receiver  string `json:"receiver" binding:"required"`
		Phone     string `json:"phone" binding:"required"`
		Province  string `json:"province" binding:"required"`
		City      string `json:"city" binding:"required"`
		District  string `json:"district" binding:"required"`
		Detail    string `json:"detail" binding:"required"`
		IsDefault bool   `json:"is_default"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, err.Error())
		return
	}

	grpcResp, err := global.AddressClient.UpdateAddress(c.Request.Context(), &pb.UpdateAddressRequest{
		Id:        int64(req.Id),
		Receiver:  req.Receiver,
		Phone:     req.Phone,
		Province:  req.Province,
		City:      req.City,
		District:  req.District,
		Detail:    req.Detail,
		IsDefault: req.IsDefault,
	})

	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	resp.OK(c, grpcResp.Data)
}

func GetAddress(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, 10001, "id 参数错误")
		return
	}

	grpcResp, err := global.AddressClient.GetAddress(c.Request.Context(), &pb.GetAddressRequest{
		Id: id,
	})

	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	resp.OK(c, grpcResp.Data)
}

func ListAddress(c *gin.Context) {
	// 从 JWT 中获取用户 ID （中间件已经解析好了）
	userId, exists := c.Get("userID")
	if !exists {
		resp.Error(c, 10001, "用户未登录")
		return
	}

	uid, ok := userId.(uint)
	if !ok {
		resp.Error(c, 10001, "用户ID类型错误")
		return
	}

	grpcResp, err := global.AddressClient.ListAddress(c.Request.Context(), &pb.ListAddressRequest{
		UserId: int64(uid),
	})

	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	resp.OK(c, grpcResp.Data)
}
