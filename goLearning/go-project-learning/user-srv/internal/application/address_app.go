package application

import (
	"context"

	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/internal/repository"
	"go.uber.org/zap"
)

// AddressApp 地址业务逻辑层
//
// 为什么 addressRepo 的类型从 *db.AddressRepo 改为 repository.AddressRepository？
// → 和 UserApp 同样的道理——依赖抽象而非具体。
//
//	AddressApp 不需要知道地址是存在 MySQL 还是 MongoDB 还是内存里。
//	它只需要知道"有个东西能帮我 CRUD 地址"。
type AddressApp struct {
	addressRepo repository.AddressRepository
}

type UpdateAddressReq struct {
	ID        uint
	Receiver  string
	Phone     string
	Province  string
	City      string
	District  string
	Detail    string
	IsDefault bool
}

func NewAddressApp(addressRepo repository.AddressRepository) *AddressApp {
	return &AddressApp{addressRepo: addressRepo}
}

func (a *AddressApp) CreateAddress(ctx context.Context, addr *entity.Address) (*entity.Address, error) {
	// 如果设置为默认地址，需要把其他地址的默认值状态取消
	if addr.IsDefault {
		if err := a.clearDefault(ctx, addr.UserID); err != nil {
			return nil, err
		}
	}

	return a.addressRepo.Create(ctx, addr)
}

func (a *AddressApp) clearDefault(ctx context.Context, userID uint) error {
	addresses, err := a.addressRepo.ListByUserID(ctx, userID)
	if err != nil {
		return err
	}
	for _, addr := range addresses {
		if addr.IsDefault {
			addr.IsDefault = false
			if _, err := a.addressRepo.Update(ctx, addr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *AddressApp) DeleteAddress(ctx context.Context, id uint) error {
	return a.addressRepo.Delete(ctx, id)
}

func (a *AddressApp) UpdateAddress(ctx context.Context, req *UpdateAddressReq) (*entity.Address, error) {
	addr, err := a.addressRepo.GetByID(ctx, req.ID)
	if err != nil {
		logger.Error("地址不存在", zap.Error(err))
		return nil, err
	}

	if req.Receiver != "" {
		addr.Receiver = req.Receiver
	}

	if req.Phone != "" {
		addr.Phone = req.Phone
	}

	if req.Province != "" {
		addr.Province = req.Province
	}

	if req.City != "" {
		addr.City = req.City
	}

	if req.District != "" {
		addr.District = req.District
	}

	if req.Detail != "" {
		addr.Detail = req.Detail
	}

	// 如果设置为默认地址，需要把其他地址的默认值状态取消
	if !addr.IsDefault && req.IsDefault {
		addr.IsDefault = req.IsDefault
		if err := a.clearDefault(ctx, addr.UserID); err != nil {
			return nil, err
		}
	}

	return a.addressRepo.Update(ctx, addr)
}

func (a *AddressApp) GetAddress(ctx context.Context, id uint) (*entity.Address, error) {
	return a.addressRepo.GetByID(ctx, id)
}

func (a *AddressApp) ListAddress(ctx context.Context, userId uint) ([]*entity.Address, error) {
	return a.addressRepo.ListByUserID(ctx, userId)
}
