package traceclient

import (
	"os"
	"path/filepath"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type artifactResolver struct {
	dir string
}

func NewArtifactResolver(dir string) port.TraceClientArtifactResolver {
	return &artifactResolver{dir: dir}
}

func (r *artifactResolver) Resolve(osName, arch string) (*port.TraceClientArtifact, error) {
	name := artifactName(osName, arch)
	if name == "" {
		return nil, ierr.New(ierr.ErrValidation, constant.TraceClientUnsupportedTarget)
	}
	path := filepath.Join(r.dir, name)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, ierr.New(ierr.ErrDataNotExists, constant.TraceClientArtifactUnavailable)
	}
	return &port.TraceClientArtifact{
		Path:     path,
		Filename: constant.TraceClientDownloadFilename,
		Size:     info.Size(),
	}, nil
}

func artifactName(osName, arch string) string {
	switch osName {
	case constant.TraceClientOSDarwin:
		switch arch {
		case constant.TraceClientArchAMD64:
			return constant.TraceClientArtifactDarwinAMD64
		case constant.TraceClientArchARM64:
			return constant.TraceClientArtifactDarwinARM64
		}
	case constant.TraceClientOSLinux:
		switch arch {
		case constant.TraceClientArchAMD64:
			return constant.TraceClientArtifactLinuxAMD64
		case constant.TraceClientArchARM64:
			return constant.TraceClientArtifactLinuxARM64
		}
	}
	return ""
}
