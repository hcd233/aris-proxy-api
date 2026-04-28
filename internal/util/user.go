package util

import (
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

var (
	minNameLen = 2
	maxNameLen = 20

	specialChars = "!#$%^&*()_+|~-=`{}[]:\";'<>?,./"
)

var specialNameblackList = []string{
	constant.UserNameBlacklistAdmin,
	constant.UserNameBlacklistRoot,
	constant.UserNameBlacklistAdministrator,
	constant.UserNameBlacklistSuperuser,
	constant.UserNameBlacklistMe,
}

// ValidateUserName 验证用户名
//
//	param userName string
//	return err error
//	author centonhuang
//	update 2024-12-09 17:22:52
func ValidateUserName(userName string) (err error) {
	validateFuncs := []func(string) error{
		validateUserNameLength,
		validateUserNameSpecialChars,
		validateUserNameSpecialName,
	}

	for _, f := range validateFuncs {
		if err = f(userName); err != nil {
			return
		}
	}

	return
}

func validateUserNameLength(userName string) error {
	if len(userName) < minNameLen || len(userName) > maxNameLen {
		return ierr.Newf(ierr.ErrValidation, "user name length must be %d-%d", minNameLen, maxNameLen)
	}
	return nil
}

func validateUserNameSpecialChars(userName string) error {
	for _, c := range userName {
		if strings.ContainsRune(specialChars, c) {
			return ierr.New(ierr.ErrValidation, "user name can't contain special characters")
		}
	}
	return nil
}

func validateUserNameSpecialName(userName string) error {
	for _, specialName := range specialNameblackList {
		if strings.EqualFold(userName, specialName) {
			return ierr.Newf(ierr.ErrValidation, "user name can't be %s", specialName)
		}
	}
	return nil
}
