package trace

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
	return Paths{Root: filepath.Join(home, constant.ArisClientRootDirName)}, nil
}

func (p Paths) BinDir() string {
	return filepath.Join(p.Root, constant.ArisClientBinDirName)
}

func (p Paths) TraceDir() string {
	return filepath.Join(p.Root, constant.ArisClientTraceDirName)
}

func (p Paths) ConfigFile() string {
	return filepath.Join(p.TraceDir(), constant.ArisClientConfigFileName)
}

func (p Paths) SpoolDir() string {
	return filepath.Join(p.TraceDir(), constant.ArisClientSpoolDirName)
}

func (p Paths) PendingDir() string {
	return filepath.Join(p.SpoolDir(), constant.ArisClientPendingDirName)
}

func (p Paths) StateDir() string {
	return filepath.Join(p.TraceDir(), constant.ArisClientStateDirName)
}

func (p Paths) RejectedDir() string {
	return filepath.Join(p.TraceDir(), constant.ArisClientRejectedDirName)
}

func (p Paths) LogDir() string {
	return filepath.Join(p.TraceDir(), constant.ArisClientLogDirName)
}

func (p Paths) CodexDir() string {
	return filepath.Join(filepath.Dir(p.Root), constant.ArisClientCodexDirName)
}

func (p Paths) CodexHooksFile() string {
	return filepath.Join(p.CodexDir(), constant.ArisClientCodexHooksFile)
}

func (p Paths) CodexHooksBackupFile() string {
	return p.CodexHooksFile() + constant.ArisClientCodexBackupSuffix
}

func (p Paths) ClaudeDir() string {
	return filepath.Join(filepath.Dir(p.Root), constant.ArisClientClaudeDirName)
}

func (p Paths) ClaudeSettingsFile() string {
	return filepath.Join(p.ClaudeDir(), constant.ArisClientClaudeSettingsFile)
}

func (p Paths) ClaudeSettingsBackupFile() string {
	return p.ClaudeSettingsFile() + constant.ArisClientCodexBackupSuffix
}
