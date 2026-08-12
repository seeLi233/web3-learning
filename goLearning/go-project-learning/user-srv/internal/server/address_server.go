package server

import (
	"context"
	"fmt"

	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-project-learning/project/user-srv/internal/application"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"go.uber.org/zap"
)

type AddressServer struct {
	pb.UnimplementedAddressServiceServer
	addressApp *application.AddressApp
}

func NewAddressServer(addressApp *application.AddressApp) *AddressServer {
	return &AddressServer{
		addressApp: addressApp,
	}
}

func (s *AddressServer) CreateAddress(ctx context.Context, req *pb.CreateAddressRequest) (*pb.CreateAddressResponse, error) {
	addr, err := s.addressApp.CreateAddress(ctx, &entity.Address{
		UserID:    uint(req.UserId),
		Receiver:  req.Receiver,
		Phone:     req.Phone,
		Province:  req.Province,
		City:      req.City,
		District:  req.District,
		Detail:    req.Detail,
		IsDefault: req.IsDefault,
	})

	if err != nil {
		return &pb.CreateAddressResponse{Code: 50001, Msg: "地址保存失败"}, err
	}

	return &pb.CreateAddressResponse{Code: 0, Msg: "地址保存成功", Data: &pb.Address{
		Id:        int64(addr.ID),
		UserId:    int64(addr.UserID),
		Receiver:  addr.Receiver,
		Phone:     addr.Phone,
		Province:  addr.Province,
		City:      addr.City,
		District:  addr.District,
		Detail:    addr.Detail,
		IsDefault: addr.IsDefault,
	}}, nil
}

func (s *AddressServer) DeleteAddress(ctx context.Context, req *pb.DeleteAddressRequest) (*pb.DeleteAddressResponse, error) {
	err := s.addressApp.DeleteAddress(ctx, uint(req.Id))
	if err != nil {
		logger.Error("删除地址失败", zap.Error(err))
		return &pb.DeleteAddressResponse{Code: 50001, Msg: "地址删除失败"}, err
	}

	return &pb.DeleteAddressResponse{Code: 0, Msg: fmt.Sprintf("地址删除成功：%d", req.Id)}, nil
}

func (s *AddressServer) UpdateAddress(ctx context.Context, req *pb.UpdateAddressRequest) (*pb.UpdateAddressResponse, error) {

	addr, err := s.addressApp.UpdateAddress(ctx, &application.UpdateAddressReq{
		ID:        uint(req.Id),
		Receiver:  req.Receiver,
		Phone:     req.Phone,
		Province:  req.Province,
		City:      req.City,
		District:  req.District,
		Detail:    req.Detail,
		IsDefault: req.IsDefault,
	})
	if err != nil {
		return &pb.UpdateAddressResponse{Code: 50001, Msg: "地址更新失败"}, err
	}
	return &pb.UpdateAddressResponse{Code: 0, Msg: "地址更新成功", Data: &pb.Address{
		Id:        int64(addr.ID),
		UserId:    int64(addr.UserID),
		Receiver:  addr.Receiver,
		Phone:     addr.Phone,
		Province:  addr.Province,
		City:      addr.City,
		District:  addr.District,
		Detail:    addr.Detail,
		IsDefault: addr.IsDefault,
	}}, nil

}

func (s *AddressServer) GetAddress(ctx context.Context, req *pb.GetAddressRequest) (*pb.GetAddressResponse, error) {
	addr, err := s.addressApp.GetAddress(ctx, uint(req.Id))
	if err != nil {
		return &pb.GetAddressResponse{Code: 50001, Msg: "地址不存在"}, err
	}

	return &pb.GetAddressResponse{Code: 0, Msg: "地址查找成功", Data: &pb.Address{
		Id:        int64(addr.ID),
		UserId:    int64(addr.UserID),
		Receiver:  addr.Receiver,
		Phone:     addr.Phone,
		Province:  addr.Province,
		City:      addr.City,
		District:  addr.District,
		Detail:    addr.Detail,
		IsDefault: addr.IsDefault,
	}}, nil
}

func (s *AddressServer) ListAddress(ctx context.Context, req *pb.ListAddressRequest) (*pb.ListAddressResponse, error) {
	addresses, err := s.addressApp.ListAddress(ctx, uint(req.UserId))
	if err != nil {
		return &pb.ListAddressResponse{Code: 50001, Msg: "地址不存在"}, err
	}

	var addrs []*pb.Address = make([]*pb.Address, len(addresses))
	for index, address := range addresses {
		addrs[index] = &pb.Address{
			Id:        int64(address.ID),
			UserId:    int64(address.UserID),
			Receiver:  address.Receiver,
			Phone:     address.Phone,
			Province:  address.Province,
			City:      address.City,
			District:  address.District,
			Detail:    address.Detail,
			IsDefault: address.IsDefault,
		}
	}
	return &pb.ListAddressResponse{Code: 0, Msg: "查询成功", Data: addrs}, nil
}
