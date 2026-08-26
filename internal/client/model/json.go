package model

import (
	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

func sonicUnmarshal(data []byte, dst any) error {
	if err := sonic.Unmarshal(data, dst); err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "decode config file")
	}
	return nil
}

func sonicMarshal(v any) ([]byte, error) {
	data, err := sonic.Marshal(v)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDTOMarshal, err, "encode config file")
	}
	return data, nil
}
