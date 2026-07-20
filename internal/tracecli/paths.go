package tracecli

import (
	"os"
	"path/filepath"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type Paths struct {
	Root string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, ierr.Wrap(ierr.ErrInternal, err, "resolve user home directory")
	}
	return Paths{Root: filepath.Join(home, constant.TraceClientRootDirName)}, nil
}

func (p Paths) BinDir() string {
	return filepath.Join(p.Root, constant.TraceClientBinDirName)
}

func (p Paths) BinFile() string {
	return filepath.Join(p.BinDir(), constant.TraceClientBinaryFileName)
}

func (p Paths) TraceDir() string {
	return filepath.Join(p.Root, constant.TraceClientTraceDirName)
}

func (p Paths) ConfigFile() string {
	return filepath.Join(p.TraceDir(), constant.TraceClientConfigFileName)
}

func (p Paths) SpoolDir() string {
	return filepath.Join(p.TraceDir(), constant.TraceClientSpoolDirName)
}

func (p Paths) PendingDir() string {
	return filepath.Join(p.SpoolDir(), constant.TraceClientPendingDirName)
}

func (p Paths) StateDir() string {
	return filepath.Join(p.TraceDir(), constant.TraceClientStateDirName)
}

func (p Paths) RejectedDir() string {
	return filepath.Join(p.TraceDir(), constant.TraceClientRejectedDirName)
}

func (p Paths) LogDir() string {
	return filepath.Join(p.TraceDir(), constant.TraceClientLogDirName)
}

func (p Paths) CodexDir() string {
	return filepath.Join(filepath.Dir(p.Root), constant.TraceClientCodexDirName)
}

func (p Paths) CodexHooksFile() string {
	return filepath.Join(p.CodexDir(), constant.TraceClientCodexHooksFile)
}

func (p Paths) CodexHooksBackupFile() string {
	return p.CodexHooksFile() + constant.TraceClientCodexBackupSuffix
}
