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
	protoc.Plugins().MustRegisterPlugin(&protocGenGrpcOpenAPIv2Plugin{})
}

// protocGenGrpcOpenAPIv2Plugin implements Plugin for protoc-gen-grpc-openapiv2
type protocGenGrpcOpenAPIv2Plugin struct{}

// Name implements part of the Plugin interface.
func (p *protocGenGrpcOpenAPIv2Plugin) Name() string {
	return "grpc-ecosystem:grpc-gateway:protoc-gen-grpc-openapiv2"
}

// Configure implements part of the Plugin interface.
func (p *protocGenGrpcOpenAPIv2Plugin) Configure(ctx *protoc.PluginContext) *protoc.PluginConfiguration {
	if !p.shouldApply(ctx.ProtoLibrary) {
		return nil
	}
	options := ctx.PluginConfig.GetOptions()
	mappings, _ := protobuf.GetImportMappings(options)
	return &protoc.PluginConfiguration{
		Label:   label.New("build_stack_rules_proto", "plugin/grpc-ecosystem/grpc-gateway", "protoc-gen-grpc-openapiv2"),
		Outputs: p.outputs(ctx.ProtoLibrary, mappings),
		Options: options,
	}
}

func (p *protocGenGrpcOpenAPIv2Plugin) shouldApply(lib protoc.ProtoLibrary) bool {
	for _, f := range lib.Files() {
		if appliesToFile(f) {
			return true
		}
	}
	return false
}

func (p *protocGenGrpcOpenAPIv2Plugin) outputs(lib protoc.ProtoLibrary, importMappings map[string]string) []string {
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
		srcs = append(srcs, base+".swagger.json")
	}
	return srcs
}

func (p *protocGenGrpcOpenAPIv2Plugin) ResolvePluginOptions(cfg *protoc.PluginConfiguration, r *rule.Rule, from label.Label) []string {
	return protobuf.ResolvePluginOptionsTransitive(cfg, r, from)
}
