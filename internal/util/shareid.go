// Package util shareID 短码生成工具
package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// GenerateShareID 基于 sessionID + 纳秒时间戳 + 随机熵进行 SHA-256 散列，
// 取前 length 个字节映射到 [0-9A-Za-z] 字符集，得到长度为 length 的短码。
// 多次调用即使输入相同的 sessionID 也会返回不同结果（随机熵保证发散性）。
//
//	@param sessionID uint
//	@param length int 输出短码长度，必须在 [constant.ShareIDMinLen, constant.ShareIDMaxLen] 之间
//	@return string
//	@return error
//	@author centonhuang
//	@update 2026-05-28 20:10:00
func GenerateShareID(sessionID uint, length int) (string, error) {
	if length < constant.ShareIDMinLen || length > constant.ShareIDMaxLen {
		return "", ierr.New(ierr.ErrInternal, fmt.Sprintf("invalid shareID length: %d", length))
	}

	// 16 字节随机熵：足够避免可预测，又能在重试时改变散列输入
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "failed to read random nonce for shareID")
	}

	// 散列输入：sessionID(8B 大端) || nanoTs(8B 大端) || nonce(16B)
	buf := make([]byte, 0, 8+8+len(nonce))
	var sessionBuf [8]byte
	binary.BigEndian.PutUint64(sessionBuf[:], uint64(sessionID))
	buf = append(buf, sessionBuf[:]...)
	var tsBuf [8]byte
	binary.BigEndian.PutUint64(tsBuf[:], uint64(time.Now().UnixNano()))
	buf = append(buf, tsBuf[:]...)
	buf = append(buf, nonce...)

	digest := sha256.Sum256(buf)

	// 取前 length 个字节，每字节模 62 映射到字符集；对短长度（6-8）足够均匀。
	out := make([]byte, length)
	for i := 0; i < length; i++ {
		out[i] = constant.ShareIDAlphabet[int(digest[i])%len(constant.ShareIDAlphabet)]
	}
	return string(out), nil
}

// IsValidShareIDChar 判断字符是否在 shareID 字符集 [0-9A-Za-z] 内
//
//	@param c byte
//	@return bool
//	@author centonhuang
//	@update 2026-05-28 20:10:00
func IsValidShareIDChar(c byte) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z')
}
