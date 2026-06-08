package server

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func (b responseBuilder) adsConfigSource() *corev3.ConfigSource {
	return &corev3.ConfigSource{
		ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
			Ads: &corev3.AggregatedConfigSource{},
		},
		ResourceApiVersion: corev3.ApiVersion_V3,
	}
}

func (b responseBuilder) socketAddress(address string, port int) *corev3.Address {
	return &corev3.Address{
		Address: &corev3.Address_SocketAddress{
			SocketAddress: &corev3.SocketAddress{
				Address: address,
				PortSpecifier: &corev3.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		},
	}
}

func (b responseBuilder) mustAny(message proto.Message) *anypb.Any {
	value, err := anypb.New(message)
	if err != nil {
		panic(err)
	}
	return value
}
