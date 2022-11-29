package grpcgateway

import (
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/stackb/rules_proto/pkg/plugin/golang/protobuf"
	"github.com/stackb/rules_proto/pkg/protoc"
	"path"
	"strings"
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
	options := ctx.PluginConfig.GetOptions()
	mappings, _ := protobuf.GetImportMappings(options)
	return &protoc.PluginConfiguration{
		Label:   label.New("build_stack_rules_proto", "plugin/grpc-ecosystem/grpc-gateway", "protoc-gen-grpc-gateway"),
		Outputs: p.outputs(ctx.ProtoLibrary, mappings),
		Options: options,
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

func (p *protocGenGrpcGatewayPlugin) outputs(lib protoc.ProtoLibrary, importMappings map[string]string) []string {
	srcs := make([]string, 0)
	for _, f := range lib.Files() {
		if !appliesToFile(f) {
			continue
		}
		base := f.Name
		pkg := f.Package()
		if mapping := importMappings[path.Join(f.Dir, f.Basename)]; mapping != "" {
			base = path.Join(mapping, base)
		} else {
			base = path.Join(strings.ReplaceAll(pkg.Name, ".", "/"), base)
		}
		srcs = append(srcs, base+".pb.gw.go")
	}
	return srcs
}

func (p *protocGenGrpcGatewayPlugin) ResolvePluginOptions(cfg *protoc.PluginConfiguration, r *rule.Rule, from label.Label) []string {
	return protobuf.ResolvePluginOptionsTransitive(cfg, r, from)
}
