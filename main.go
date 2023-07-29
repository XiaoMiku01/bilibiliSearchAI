package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/XiaoMiku01/bilibili-grpc-api-go/bilibili/metadata/device"
	"github.com/XiaoMiku01/bilibili-grpc-api-go/bilibili/metadata/locale"
	"github.com/XiaoMiku01/bilibili-grpc-api-go/bilibili/metadata/network"
	"github.com/XiaoMiku01/bilibili-grpc-api-go/bilibili/rpc"

	searchapi "github.com/XiaoMiku01/bilibili-grpc-api-go/bilibili/app/search/v2"
	chat "github.com/XiaoMiku01/bilibili-grpc-api-go/bilibili/broadcast/message/main"
	bilimetadata "github.com/XiaoMiku01/bilibili-grpc-api-go/bilibili/metadata"
)

func getBiliBiliMetaData(accessKey string) metadata.MD {
	device := &device.Device{
		MobiApp:  "android",
		Device:   "phone",
		Build:    6830300,
		Channel:  "bili",
		Buvid:    "XX82B818F96FB2F312B3A1BA44DB41892FF99",
		Platform: "android",
	}
	devicebin, _ := proto.Marshal(device)
	locale := &locale.Locale{
		Timezone: "Asia/Shanghai",
	}
	localebin, _ := proto.Marshal(locale)
	bilimetadata := &bilimetadata.Metadata{
		AccessKey: accessKey,
		MobiApp:   "android",
		Device:    "phone",
		Build:     6830300,
		Channel:   "bili",
		Buvid:     "XX82B818F96FB2F312B3A1BA44DB41892FF99",
		Platform:  "android",
	}
	bilimetadatabin, _ := proto.Marshal(bilimetadata)
	network := &network.Network{
		Type: network.NetworkType_WIFI,
	}
	networkbin, _ := proto.Marshal(network)
	md := metadata.Pairs(
		"x-bili-device-bin", string(devicebin),
		"x-bili-local-bin", string(localebin),
		"x-bili-metadata-bin", string(bilimetadatabin),
		"x-bili-network-bin", string(networkbin),
		"authorization", "identify_v1 "+accessKey,
	)
	return md
}
func grpcErr(err error) {
	status, ok := status.FromError(err)
	if !ok {
		log.Printf("BiliGRPC error: %v", err)
		return
	}
	// B站的grpc接口返回的错误码 例如鉴权错误
	if status.Code() == codes.Unknown && len(status.Details()) > 0 {
		rpc, ok := status.Details()[0].(*rpc.Status)
		if !ok {
			log.Printf("BiliGRPC  error0: %v", err)
			return
		}
		log.Println(rpc.Code, rpc.Message)
		return
	} else {
		log.Printf("BiliGRPC  error1: %v", err)
		return
	}
}

var grpcClient *grpc.ClientConn

func init() {
	addr := "grpc.biliapi.net:443"
	creds := grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12,
		//InsecureSkipVerify: true,
	}))
	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: true,
	}
	var err error
	options := []grpc.DialOption{
		creds,
		grpc.WithKeepaliveParams(kacp),
	}
	grpcClient, err = grpc.Dial(addr, options...)
	if err != nil {
		log.Fatalf("BiliGRPC did not connect: %v", err)

	}
	log.Println("BiliGRPC connect success")
}

func searchChat(q string) (string, error) {
	accessKey := ""
	md := getBiliBiliMetaData(accessKey)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	searchClient := searchapi.NewSearchClient(grpcClient)
	chatSubReq := &searchapi.SubmitChatTaskReq{
		Query: q,
	}

	chatSubResp, err := searchClient.SubmitChatTask(ctx, chatSubReq)
	if err != nil {
		grpcErr(err)
		return "", err
	}

	//log.Println(chatSubResp)
	chatGetReq := searchapi.GetChatResultReq{
		Query:     q,
		SessionId: chatSubResp.SessionId,
	}
	var chatResult *chat.ChatResult
	var retry = 0
	for chatResult == nil {
		chatResult, err = searchClient.GetChatResult(ctx, &chatGetReq)
		if err != nil {
			//grpcErr(err)
			if retry > 10 {
				return "", fmt.Errorf("回答超时")
			}
			time.Sleep(time.Millisecond * 1000)
			log.Println("waiting ...")
			retry++
			continue
		}
	}
	if len(chatResult.Bubble) < 2 {
		log.Println("no bubble")
		return "", fmt.Errorf("没有回答")
	}
	texts := chatResult.Bubble[0]
	var result string
	for _, text := range texts.Paragraphs {
		ts := text.GetText()
		for _, node := range ts.Nodes {
			// fmt.Print(node.RawText)
			result += node.RawText
		}
	}
	return result, nil
}

func main() {
	for {
		var ask string
		fmt.Print("请输入问题：")
		fmt.Scanln(&ask)
		result, err := searchChat(ask)
		if err != nil {
			log.Println("回答出错：", err)
			fmt.Println("--------------------")
			continue
		}
		fmt.Println("bilibiliSearchAI回答：", result)
		fmt.Println("--------------------")
	}
}
