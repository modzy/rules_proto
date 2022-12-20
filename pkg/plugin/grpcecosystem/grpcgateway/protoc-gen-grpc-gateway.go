package grpcgateway

import (
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/stackb/rules_proto/pkg/protoc"
	"path"
)

func init() {
	protoc.Plugins().MustRegisterPlugin(&protocGenGrpcGatewayPlugin{})
}

// protocGenGrpcGatewayPlugin implements Plugin for protoc-gen-grpc-gateway.
type protocGenGrpcGatewayPlugin struct{}

// Name implements part of the Plugin interface.
func (p *protocGenGrpcGatewayPlugin) Name() string {
	return "grpc-ecosystem:grpc-gateway:protoc-gen-grpc-gateway"
}

// Configure implements part of the Plugin interface.
func (p *protocGenGrpcGatewayPlugin) Configure(ctx *protoc.PluginContext) *protoc.PluginConfiguration {
	if !p.shouldApply(ctx.ProtoLibrary) {
		return nil
	}
	return &protoc.PluginConfiguration{
		Label:   label.New("build_stack_rules_proto", "plugin/grpc-ecosystem/grpc-gateway", "protoc-gen-grpc-gateway"),
		Outputs: p.outputs(ctx.Rel, ctx.ProtoLibrary),
		Options: ctx.PluginConfig.GetOptions(),
	}
}

func (p *protocGenGrpcGatewayPlugin) shouldApply(lib protoc.ProtoLibrary) bool {
	for _, f := range lib.Files() {
		if appliesToFile(f) {
			return true
		}
	}
	return false
}

func appliesToFile(f *protoc.File) bool {
	// If this proto doesn't have any services, then skip it.
	if f.HasServices() {
		// Not all protos that have services support gRPC Gateway.
		// Look to see if the proto imports "google/api/annotations.proto" as a
		// proxy to guess whether we should apply to this file or not.
		for _, i := range f.Imports() {
			if i.Filename == "google/api/annotations.proto" {
				return true
			}
		}
	}
	return false
}

func (p *protocGenGrpcGatewayPlugin) outputs(rel string, lib protoc.ProtoLibrary) []string {
	srcs := make([]string, 0)
	for _, f := range lib.Files() {
		if !appliesToFile(f) {
			continue
		}
		base := f.Name
		if rel != "" {
			base = path.Join(rel, base)
		}
		srcs = append(srcs, base+".pb.gw.go")
	}
	return srcs
}
