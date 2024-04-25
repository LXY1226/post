package shared

import (
	"context"

	pb "github.com/spacemeshos/api/release/go/spacemesh/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func GetHighestAtxId(ctx context.Context, nodeAddr string) ([]byte, error) {
	conn, err := grpc.NewClient(nodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	var reply pb.HighestResponse
	err = conn.Invoke(ctx, "/spacemesh.v1.ActivationService/Highest", &emptypb.Empty{}, &reply)
	if err != nil {
		return nil, err
	}
	return reply.Atx.Id.Id, nil
}
