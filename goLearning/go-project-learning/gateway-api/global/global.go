package global

import (
	pb "github.com/go-project-learning/project/user-srv/api/pb"
)

var (
	UserClient    pb.UserServiceClient
	OAuthClient   pb.OAuthServiceClient
	AddressClient pb.AddressServiceClient
	MemberClient  pb.MemberServiceClient
	CouponClient  pb.CouponServiceClient
	RiskClient    pb.RiskServiceClient
)
