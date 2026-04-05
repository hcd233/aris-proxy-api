package dao

var (
	userDAOSingleton          *UserDAO
	messageDAOSingleton       *MessageDAO
	sessionDAOSingleton       *SessionDAO
	toolDAOSingleton          *ToolDAO
	modelEndpointDAOSingleton *ModelEndpointDAO
	proxyAPIKeyDAOSingleton   *ProxyAPIKeyDAO
)

func init() {
	userDAOSingleton = &UserDAO{}
	messageDAOSingleton = &MessageDAO{}
	sessionDAOSingleton = &SessionDAO{}
	toolDAOSingleton = &ToolDAO{}
	modelEndpointDAOSingleton = &ModelEndpointDAO{}
	proxyAPIKeyDAOSingleton = &ProxyAPIKeyDAO{}
}

// GetUserDAO 获取用户DAO
//
//	@return *UserDAO
//	@author centonhuang
//	@update 2024-10-17 04:59:37
func GetUserDAO() *UserDAO {
	return userDAOSingleton
}

// GetMessageDAO 获取消息DAO
//
//	@return *MessageDAO
//	@author centonhuang
//	@update 2026-03-10 10:00:00
func GetMessageDAO() *MessageDAO {
	return messageDAOSingleton
}

// GetSessionDAO 获取会话DAO
//
//	@return *SessionDAO
//	@author centonhuang
//	@update 2026-03-10 10:00:00
func GetSessionDAO() *SessionDAO {
	return sessionDAOSingleton
}

// GetToolDAO 获取工具DAO
//
//	@return *ToolDAO
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func GetToolDAO() *ToolDAO {
	return toolDAOSingleton
}

// GetModelEndpointDAO 获取模型端点DAO
//
//	@return *ModelEndpointDAO
//	@author centonhuang
//	@update 2026-04-04 10:00:00
func GetModelEndpointDAO() *ModelEndpointDAO {
	return modelEndpointDAOSingleton
}

// GetProxyAPIKeyDAO 获取代理API密钥DAO
//
//	@return *ProxyAPIKeyDAO
//	@author centonhuang
//	@update 2026-04-04 10:00:00
func GetProxyAPIKeyDAO() *ProxyAPIKeyDAO {
	return proxyAPIKeyDAOSingleton
}
