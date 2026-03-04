package dao

var userDAOSingleton *UserDAO

func init() {
	userDAOSingleton = &UserDAO{}
}

// GetUserDAO 获取用户DAO
//
//	return *baseDAO
//	author centonhuang
//	update 2024-10-17 04:59:37
func GetUserDAO() *UserDAO {
	return userDAOSingleton
}
