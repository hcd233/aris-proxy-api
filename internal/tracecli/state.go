package tracecli

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type clientState struct {
	SpoolID      string `json:"spoolID"`
	NextSequence int64  `json:"nextSequence"`
}

func nextSequence(ctx context.Context, paths Paths) (spoolID string, sequence int64, err error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", 0, ctxErr
	}
	err = withFileLock(filepath.Join(paths.StateDir(), constant.TraceClientStateLockFile), func() error {
		var state clientState
		state, err = loadClientState(paths)
		if err != nil {
			return err
		}
		if state.SpoolID == "" {
			random := make([]byte, constant.TraceClientSpoolIDRandomBytes)
			if _, err = rand.Read(random); err != nil {
				return ierr.Wrap(ierr.ErrInternal, err, "generate trace spool id")
			}
			state.SpoolID = base64.RawURLEncoding.EncodeToString(random)
		}
		if state.NextSequence < 1 {
			state.NextSequence = 1
		}
		spoolID = state.SpoolID
		sequence = state.NextSequence
		state.NextSequence++
		var data []byte
		data, err = sonic.Marshal(state)
		if err != nil {
			return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode trace client state")
		}
		return writePrivateFile(filepath.Join(paths.StateDir(), constant.TraceClientStateFileName), data)
	})
	return spoolID, sequence, err
}

func loadClientState(paths Paths) (clientState, error) {
	data, err := os.ReadFile(filepath.Join(paths.StateDir(), constant.TraceClientStateFileName))
	if errors.Is(err, os.ErrNotExist) {
		return clientState{}, nil
	}
	if err != nil {
		return clientState{}, ierr.Wrap(ierr.ErrInternal, err, "read trace client state")
	}
	var state clientState
	if err := sonic.Unmarshal(data, &state); err != nil {
		return clientState{}, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode trace client state")
	}
	return state, nil
}
