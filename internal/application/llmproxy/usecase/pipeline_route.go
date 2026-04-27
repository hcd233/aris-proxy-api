package usecase

import "github.com/hcd233/aris-proxy-api/internal/enum"

type pipelineRouteMode string

var (
	pipelineRouteModeUnary  = pipelineRouteMode("unary")
	pipelineRouteModeStream = pipelineRouteMode("stream")
)

type pipelineRoute struct {
	SourceProvider enum.ProviderType
	TargetProvider enum.ProviderType
	Mode           pipelineRouteMode
}

func selectPipelineRoute(sourceProvider, targetProvider enum.ProviderType, stream bool) pipelineRoute {
	mode := pipelineRouteModeUnary
	if stream {
		mode = pipelineRouteModeStream
	}
	return pipelineRoute{SourceProvider: sourceProvider, TargetProvider: targetProvider, Mode: mode}
}
