package constant

const (
	RoutePathRoot                         = "/"
	RoutePathHealth                       = "/health"
	RoutePathSSEHealth                    = "/ssehealth"
	RoutePathTokenRefresh                 = "/refresh"
	RoutePathUserCurrent                  = "/current"
	RoutePathSessionList                  = "/list"
	RoutePathOAuthLogin                   = "/login"
	RoutePathOAuthCallback                = "/callback"
	RoutePathModels                       = "/models"
	RoutePathAnthropicMessages            = "/messages"
	RoutePathAnthropicMessagesCountTokens = "/messages/count_tokens"
	RoutePathOpenAIChatCompletions        = "/chat/completions"
	RoutePathAPIKeyByID                   = "/{id}"
	RoutePathFavicon                      = "/favicon.ico"
	RoutePathRobots                       = "/robots.txt"
	RoutePathAppleTouchIcon               = "/apple-touch-icon.png"
	RoutePathAppleTouchIconPrecomposed    = "/apple-touch-icon-precomposed.png"
	RoutePathWellKnownSecurity            = "/.well-known/security.txt"

	RoutePathOAuth2Callback     = "/api/v1/oauth2/callback"
	RoutePathOAuth2ExchangeCode = "/api/v1/oauth2/exchange-code"
	RoutePathWebAuthCallback    = "/web/auth/callback"
	RoutePathWebLogin           = "/web/login"
)
