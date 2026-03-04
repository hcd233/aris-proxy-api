package util

import (
	"fmt"
	"strings"
)

const (
	minNameLen = 2
	maxNameLen = 20

	specialChars = "!#$%^&*()_+|~-=`{}[]:\";'<>?,./"
)

var specialNameblackList = []string{
	"admin", "root", "administrator", "superuser", "me",
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
		return fmt.Errorf("user name length must be %d-%d", minNameLen, maxNameLen)
	}
	return nil
}

func validateUserNameSpecialChars(userName string) error {
	for _, c := range userName {
		if strings.ContainsRune(specialChars, c) {
			return fmt.Errorf("user name can't contain special characters")
		}
	}
	return nil
}

func validateUserNameSpecialName(userName string) error {
	for _, specialName := range specialNameblackList {
		if strings.EqualFold(userName, specialName) {
			return fmt.Errorf("user name can't be %s", specialName)
		}
	}
	return nil
}
