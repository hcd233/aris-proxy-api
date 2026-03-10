package dao

var (
	userDAOSingleton    *UserDAO
	messageDAOSingleton *MessageDAO
)

func init() {
	userDAOSingleton = &UserDAO{}
	messageDAOSingleton = &MessageDAO{}
}

// GetUserDAO 获取用户DAO
//
//	return *baseDAO
//	author centonhuang
//	update 2024-10-17 04:59:37
func GetUserDAO() *UserDAO {
	return userDAOSingleton
}

// GetMessageDAO 获取消息DAO
//
//	return *MessageDAO
//	@author centonhuang
//	@update 2026-03-10 10:00:00
func GetMessageDAO() *MessageDAO {
	return messageDAOSingleton
}
