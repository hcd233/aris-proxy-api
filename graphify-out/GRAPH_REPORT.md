# Graph Report - internal  (2026-05-11)

## Corpus Check
- Large corpus: 243 files · ~72,907 words. Semantic extraction will be expensive (many Claude tokens). Consider running on a subfolder, or use --no-semantic to run AST-only.

## Summary
- 1457 nodes · 2046 edges · 180 communities (119 shown, 61 thin omitted)
- Extraction: 73% EXTRACTED · 27% INFERRED · 0% AMBIGUOUS · INFERRED: 560 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_LLM Proxy Use Cases|LLM Proxy Use Cases]]
- [[_COMMUNITY_Repository Layer|Repository Layer]]
- [[_COMMUNITY_Lint Convention|Lint Convention]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Agent Pipeline|Agent Pipeline]]
- [[_COMMUNITY_Protocol Converter|Protocol Converter]]
- [[_COMMUNITY_Application Bootstrap|Application Bootstrap]]
- [[_COMMUNITY_OAuth2|OAuth2]]
- [[_COMMUNITY_Aggregate Roots|Aggregate Roots]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_HTTP Middleware|HTTP Middleware]]
- [[_COMMUNITY_Error Handling|Error Handling]]
- [[_COMMUNITY_Protocol Converter|Protocol Converter]]
- [[_COMMUNITY_Transport Layer|Transport Layer]]
- [[_COMMUNITY_Error Handling|Error Handling]]
- [[_COMMUNITY_LLM Proxy Use Cases|LLM Proxy Use Cases]]
- [[_COMMUNITY_OAuth2|OAuth2]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Aggregate Roots|Aggregate Roots]]
- [[_COMMUNITY_Aggregate Roots|Aggregate Roots]]
- [[_COMMUNITY_Object Storage|Object Storage]]
- [[_COMMUNITY_Object Storage DAO|Object Storage DAO]]
- [[_COMMUNITY_Aggregate Roots|Aggregate Roots]]
- [[_COMMUNITY_Logging|Logging]]
- [[_COMMUNITY_Cron Jobs|Cron Jobs]]
- [[_COMMUNITY_HTTP Middleware|HTTP Middleware]]
- [[_COMMUNITY_LLM Proxy Use Cases|LLM Proxy Use Cases]]
- [[_COMMUNITY_Application Layer|Application Layer]]
- [[_COMMUNITY_HTTP Middleware|HTTP Middleware]]
- [[_COMMUNITY_Application Layer|Application Layer]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_HTTP Middleware|HTTP Middleware]]
- [[_COMMUNITY_Logging|Logging]]
- [[_COMMUNITY_Data Access Objects|Data Access Objects]]
- [[_COMMUNITY_Cron Jobs|Cron Jobs]]
- [[_COMMUNITY_Application Layer|Application Layer]]
- [[_COMMUNITY_Object Storage DAO|Object Storage DAO]]
- [[_COMMUNITY_Distributed Lock|Distributed Lock]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Cron Jobs|Cron Jobs]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Agent Pipeline|Agent Pipeline]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Aggregate Roots|Aggregate Roots]]
- [[_COMMUNITY_Common Utilities|Common Utilities]]
- [[_COMMUNITY_Vo|Vo]]
- [[_COMMUNITY_Vo|Vo]]
- [[_COMMUNITY_Vo|Vo]]
- [[_COMMUNITY_Session Management|Session Management]]
- [[_COMMUNITY_Goroutine Pool|Goroutine Pool]]
- [[_COMMUNITY_Handler|Handler]]
- [[_COMMUNITY_Aggregate Roots|Aggregate Roots]]
- [[_COMMUNITY_Aggregate Roots|Aggregate Roots]]
- [[_COMMUNITY_Handler|Handler]]
- [[_COMMUNITY_Vo|Vo]]
- [[_COMMUNITY_Goroutine Pool|Goroutine Pool]]
- [[_COMMUNITY_Cron Jobs|Cron Jobs]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Application Layer|Application Layer]]
- [[_COMMUNITY_Logging|Logging]]
- [[_COMMUNITY_Common Utilities|Common Utilities]]
- [[_COMMUNITY_Application Layer|Application Layer]]
- [[_COMMUNITY_Data Access Objects|Data Access Objects]]
- [[_COMMUNITY_Vo|Vo]]
- [[_COMMUNITY_Lint Convention|Lint Convention]]
- [[_COMMUNITY_Cron Jobs|Cron Jobs]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Handler|Handler]]
- [[_COMMUNITY_Application Layer|Application Layer]]
- [[_COMMUNITY_JWT Auth|JWT Auth]]
- [[_COMMUNITY_Configuration|Configuration]]
- [[_COMMUNITY_Llmproxy|Llmproxy]]
- [[_COMMUNITY_Cron Jobs|Cron Jobs]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Vo|Vo]]
- [[_COMMUNITY_Lint Static|Lint Static]]
- [[_COMMUNITY_Repository Layer|Repository Layer]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Enumerations|Enumerations]]
- [[_COMMUNITY_Handler|Handler]]
- [[_COMMUNITY_Handler|Handler]]
- [[_COMMUNITY_Handler|Handler]]
- [[_COMMUNITY_Handler|Handler]]
- [[_COMMUNITY_Object Storage DAO|Object Storage DAO]]
- [[_COMMUNITY_Service|Service]]
- [[_COMMUNITY_Conversation|Conversation]]
- [[_COMMUNITY_Dto|Dto]]
- [[_COMMUNITY_Enumerations|Enumerations]]
- [[_COMMUNITY_Common Utilities|Common Utilities]]
- [[_COMMUNITY_Common Utilities|Common Utilities]]
- [[_COMMUNITY_Common Utilities|Common Utilities]]
- [[_COMMUNITY_Common Utilities|Common Utilities]]
- [[_COMMUNITY_Common Utilities|Common Utilities]]
- [[_COMMUNITY_Data Access Objects|Data Access Objects]]
- [[_COMMUNITY_Data Access Objects|Data Access Objects]]
- [[_COMMUNITY_Data Access Objects|Data Access Objects]]
- [[_COMMUNITY_Data Access Objects|Data Access Objects]]
- [[_COMMUNITY_Data Access Objects|Data Access Objects]]
- [[_COMMUNITY_Data Access Objects|Data Access Objects]]
- [[_COMMUNITY_Data Access Objects|Data Access Objects]]
- [[_COMMUNITY_Data Access Objects|Data Access Objects]]
- [[_COMMUNITY_Infrastructure|Infrastructure]]
- [[_COMMUNITY_Infrastructure|Infrastructure]]
- [[_COMMUNITY_Infrastructure|Infrastructure]]
- [[_COMMUNITY_Infrastructure|Infrastructure]]
- [[_COMMUNITY_Infrastructure|Infrastructure]]
- [[_COMMUNITY_Infrastructure|Infrastructure]]
- [[_COMMUNITY_Infrastructure|Infrastructure]]
- [[_COMMUNITY_Infrastructure|Infrastructure]]
- [[_COMMUNITY_Domain|Domain]]
- [[_COMMUNITY_Domain|Domain]]
- [[_COMMUNITY_API Key Management|API Key Management]]
- [[_COMMUNITY_Domain|Domain]]
- [[_COMMUNITY_Domain|Domain]]

## God Nodes (most connected - your core abstractions)
1. `WithCtx()` - 60 edges
2. `Wrap()` - 52 edges
3. `New()` - 41 edges
4. `GetDBInstance()` - 37 edges
5. `checker` - 25 edges
6. `Logger()` - 22 edges
7. `openAIUseCase` - 22 edges
8. `GetPoolManager()` - 19 edges
9. `isUnder()` - 19 edges
10. `CopyContextValues()` - 17 edges

## Surprising Connections (you probably didn't know these)
- `InitInfrastructure()` --calls--> `InitPoolManager()`  [INFERRED]
  bootstrap/container.go → infrastructure/pool/pool.go
- `validateUserNameSpecialChars()` --calls--> `New()`  [INFERRED]
  util/user.go → common/ierr/ierr.go
- `RecordMessage()` --calls--> `New()`  [INFERRED]
  domain/conversation/aggregate/message.go → common/ierr/ierr.go
- `RecordTool()` --calls--> `New()`  [INFERRED]
  domain/conversation/aggregate/tool.go → common/ierr/ierr.go
- `toSessionAggregate()` --calls--> `RestoreSessionScore()`  [INFERRED]
  infrastructure/repository/session_repository.go → domain/session/vo/summary_score.go

## Communities (180 total, 61 thin omitted)

### Community 0 - "LLM Proxy Use Cases"
Cohesion: 0.06
Nodes (30): GenerateOpenAIChunkID(), GetPoolManager(), ReplaceModelInBody(), ReplaceModelInSSEData(), AnthropicUseCase, auditFailure(), toTransportEndpoint(), openAIUseCase (+22 more)

### Community 1 - "Repository Layer"
Cohesion: 0.05
Nodes (30): GetModelEndpointDAO(), CloseDatabase(), GetDBInstance(), InitDatabase(), Wrap(), toAPIKeyAggregate(), toAPIKeyAggregateList(), apiKeyRepository (+22 more)

### Community 2 - "Lint Convention"
Cohesion: 0.06
Nodes (41): checker, goFilePath, isDeprecatedApplicationImport(), isHandlerDBCall(), isInterfaceLayerPath(), receiverIdentName(), isConstantOrEnumPath(), containsChinese() (+33 more)

### Community 3 - "Dto"
Cohesion: 0.04
Nodes (33): ResponseApplyPatchOperation, ResponseCodeInterpreterCallOutput, ResponseComputerAction, ResponseComputerActionPathPoint, ResponseComputerCallOutputScreenshot, ResponseContainerNetworkDomainSecret, ResponseContainerNetworkPolicy, ResponseFileSearchCallResult (+25 more)

### Community 4 - "Dto"
Cohesion: 0.04
Nodes (40): newAnthropicThinkingContentBlockWire(), newAnthropicToolUseContentBlockWire(), AnthropicContentBlock, AnthropicContentBlockCaller, AnthropicContentSource, AnthropicContextManagement, AnthropicContextManagementEdit, AnthropicCountTokensReq (+32 more)

### Community 5 - "Dto"
Cohesion: 0.04
Nodes (47): OpenAIAllowedToolsConfig, OpenAIApproximateLocation, OpenAIChatCompletion, OpenAIChatCompletionAudio, OpenAIChatCompletionAudioParam, OpenAIChatCompletionAudioReference, OpenAIChatCompletionChoice, OpenAIChatCompletionChunk (+39 more)

### Community 6 - "Agent Pipeline"
Cohesion: 0.06
Nodes (14): GetScorer(), NewScorer(), ScoreResult, Summarizer, GetSummarizer(), NewSummarizer(), getSessionRepo(), NewSessionRepository() (+6 more)

### Community 7 - "Protocol Converter"
Cohesion: 0.11
Nodes (30): convertAnthropicContentToOpenAIMessage(), convertAnthropicStopReasonToOpenAI(), convertContentBlockDeltaToChunks(), convertContentBlockStartToChunks(), convertMessageDeltaToChunks(), convertOpenAIAssistantMessageToAnthropic(), convertOpenAIImageURLToAnthropicBlock(), convertOpenAIPartsToAnthropicBlocks() (+22 more)

### Community 8 - "Application Bootstrap"
Cohesion: 0.07
Nodes (13): BuildServer(), newAccessTokenSigner(), newRefreshTokenSigner(), provide(), provideApplication(), provideHandlers(), provideHTTP(), provideInfrastructure() (+5 more)

### Community 9 - "OAuth2"
Cohesion: 0.07
Nodes (11): newOauth2Platforms(), NewGithubPlatform(), GithubEmail, githubPlatform, GithubUserInfo, NewGooglePlatform(), googlePlatform, GoogleUserInfo (+3 more)

### Community 10 - "Aggregate Roots"
Cohesion: 0.07
Nodes (11): IssueProxyAPIKey(), RestoreProxyAPIKey(), ProxyAPIKey, IssueAPIKeyCommand, IssueAPIKeyHandler, IssueAPIKeyResult, UserExistenceChecker, DefaultAPIKeyQuota() (+3 more)

### Community 11 - "Dto"
Cohesion: 0.07
Nodes (28): ResponseCodeInterpreterContainerAuto, ResponseCustomToolFormat, ResponseFileSearchFilter, ResponseFileSearchHybridSearch, ResponseFileSearchRankingOptions, ResponseImageGenerationMask, ResponseMcpToolApprovalFilter, ResponseMcpToolFilter (+20 more)

### Community 12 - "HTTP Middleware"
Cohesion: 0.09
Nodes (17): routeParams, RegisterRoutes(), GetProxyAPIKeyDAO(), GetUserDAO(), APIKeyMiddleware(), HeaderPassthroughMiddleware(), TokenBucketRateLimiterMiddleware(), NewAPIKeyRepository() (+9 more)

### Community 13 - "Error Handling"
Cohesion: 0.09
Nodes (15): convertOpenAIContent(), convertOpenAIContentPart(), extractAnthropicBlocks(), FromAnthropicMessage(), FromAnthropicResponse(), FromOpenAIMessage(), FromAnthropicTool(), FromOpenAITool() (+7 more)

### Community 14 - "Protocol Converter"
Cohesion: 0.15
Nodes (19): convertAnthropicBlocksToOpenAIMessages(), convertAnthropicImageToOpenAIPart(), convertAnthropicMessageToOpenAI(), convertAnthropicSystemToOpenAI(), convertAnthropicToolChoiceToOpenAI(), convertAnthropicToolsToOpenAI(), convertChunkUsageToAnthropic(), convertOpenAIFinishReasonToAnthropic() (+11 more)

### Community 15 - "Transport Layer"
Cohesion: 0.15
Nodes (8): GetHTTPClient(), InitHTTPClient(), AnthropicProxy, capturePassthroughResponseHeaders(), isPassthroughResponseHeader(), storePassthroughResponseHeaders(), OpenAIProxy, GetPassthroughHeaders()

### Community 16 - "Error Handling"
Cohesion: 0.22
Nodes (9): APIKeyHandler, Oauth2Handler, SessionHandler, UserHandler, ToBizError(), CtxValuePermission(), CtxValueString(), CtxValueUint() (+1 more)

### Community 17 - "LLM Proxy Use Cases"
Cohesion: 0.2
Nodes (15): collectReasoningText(), fromResponseAPIFunctionCall(), fromResponseAPIFunctionCallOutput(), FromResponseAPIInputItems(), fromResponseAPIItem(), fromResponseAPIMessage(), FromResponseAPIOutputItems(), fromResponseAPIReasoning() (+7 more)

### Community 18 - "OAuth2"
Cohesion: 0.12
Nodes (11): HandleCallbackCommand, HandleCallbackResult, InitiateLoginCommand, InitiateLoginHandler, InitiateLoginResult, ObjectStorageDirCreator, GenerateOAuth2State(), NewStateManager() (+3 more)

### Community 19 - "Dto"
Cohesion: 0.12
Nodes (11): OpenAICreateResponseReq, OpenAICreateResponseRequest, ResponseContextManagementEntry, ResponseConversationParam, ResponseConversationValue, ResponsePrompt, ResponsePromptVariable, ResponseReasoningConfig (+3 more)

### Community 20 - "Aggregate Roots"
Cohesion: 0.12
Nodes (4): newAudit(), RecordCall(), ModelCallAudit, RecordCallInput

### Community 21 - "Aggregate Roots"
Cohesion: 0.12
Nodes (4): Session, CreateSession(), hasDuplicateIDs(), RestoreSession()

### Community 22 - "Object Storage"
Cohesion: 0.16
Nodes (10): createObjectStorageDAO(), GetAudioObjDAO(), NewAudioDirCreator(), AudioDirCreator, GetCosClient(), initCosClient(), GetMinioStorage(), initMinioClient() (+2 more)

### Community 23 - "Object Storage DAO"
Cohesion: 0.17
Nodes (3): Base, Root, MinioObjDAO

### Community 24 - "Aggregate Roots"
Cohesion: 0.13
Nodes (3): User, RegisterUser(), RestoreUser()

### Community 25 - "Logging"
Cohesion: 0.28
Nodes (4): HandleCallbackHandler, GormLoggerAdapter, WithCtx(), ValidateUserName()

### Community 26 - "Cron Jobs"
Cohesion: 0.22
Nodes (11): newCronLoggerAdapter(), NewSessionDeduplicateCron(), NewSessionScoreCron(), NewSessionSummarizeCron(), NewSoftDeletePurgeCron(), GetMessageDAO(), GetSessionDAO(), GetToolDAO() (+3 more)

### Community 27 - "HTTP Middleware"
Cohesion: 0.15
Nodes (7): NewHumaAPI(), GenerateAnthropicMessageID(), New(), CompressMiddleware(), CORSMiddleware(), FgprofMiddleware(), TraceMiddleware()

### Community 28 - "LLM Proxy Use Cases"
Cohesion: 0.15
Nodes (5): NewUpstreamEndpointFromCredential(), UpstreamEndpoint, CountTokens, ListAnthropicModels, ListOpenAIModels

### Community 29 - "Application Layer"
Cohesion: 0.15
Nodes (8): GetSessionHandler, GetSessionQuery, ListSessionsHandler, ListSessionsQuery, MessageView, SessionDetailView, SessionSummaryView, ToolView

### Community 30 - "HTTP Middleware"
Cohesion: 0.19
Nodes (8): JwtMiddleware(), jwtUserCacheKey(), jwtUserCache, RedisLockMiddleware(), LimitUserPermissionMiddleware(), initAPIKeyRouter(), initUserRouter(), WriteErrorResponse()

### Community 31 - "Application Layer"
Cohesion: 0.19
Nodes (5): UpdateProfileCommand, UpdateProfileHandler, Avatar, Email, UserName

### Community 32 - "Dto"
Cohesion: 0.17
Nodes (3): OpenAIChatCompletionToolChoiceParam, OpenAIMessageContent, OpenAIVoiceParam

### Community 33 - "HTTP Middleware"
Cohesion: 0.23
Nodes (8): isSensitiveHeader(), LogMiddleware(), LogMiddlewareConfig, logSampler, LogSamplingRule, TruncateFieldValue(), TruncateMapValues(), truncateValue()

### Community 34 - "Logging"
Cohesion: 0.21
Nodes (4): encodeFields(), valueToString(), clsCallback, clsCore

### Community 36 - "Cron Jobs"
Cohesion: 0.23
Nodes (7): MergeResult, FindRedundantSessions(), FindRedundantSessionsWithMerge(), isEqualSlice(), IsSubArray(), SessionDeduplicateCron, Sort

### Community 37 - "Application Layer"
Cohesion: 0.18
Nodes (4): RefreshTokensCommand, RefreshTokensHandler, NewTokenPair(), TokenPair

### Community 39 - "Distributed Lock"
Cohesion: 0.2
Nodes (7): GetRedisClient(), NewLocker(), Locker, redisLocker, GuardMiddleware(), isRouteNotFound(), GuardConfig

### Community 40 - "Dto"
Cohesion: 0.2
Nodes (5): MessageStoreTask, ModelCallAuditTask, PingTask, ScoreTask, SummarizeTask

### Community 41 - "Cron Jobs"
Cohesion: 0.31
Nodes (6): Cron, capitalizeFirst(), convertZapKeyValues(), StopCronJobs(), cronLoggerAdapter, CronRegistryEntry

### Community 42 - "Dto"
Cohesion: 0.22
Nodes (8): GetSessionReq, GetSessionRsp, ListSessionsReq, ListSessionsRsp, MessageItem, SessionDetail, SessionSummary, ToolItem

### Community 44 - "Agent Pipeline"
Cohesion: 0.31
Nodes (5): Scorer, InitInfrastructure(), InitCache(), InitCronJobs(), Logger()

### Community 45 - "Dto"
Cohesion: 0.25
Nodes (7): OpenAICreateResponseRsp, ResponseErrorBody, ResponseIncomplete, ResponseInputTokensDetail, ResponseOutputTokensDetail, ResponseStreamTerminalEvent, ResponseUsage

### Community 46 - "Dto"
Cohesion: 0.25
Nodes (7): APIKeyDetail, APIKeyItem, CreateAPIKeyReq, CreateAPIKeyReqBody, CreateAPIKeyRsp, DeleteAPIKeyReq, ListAPIKeysRsp

### Community 47 - "Aggregate Roots"
Cohesion: 0.25
Nodes (3): Message, RecordMessage(), RestoreMessage()

### Community 48 - "Common Utilities"
Cohesion: 0.25
Nodes (3): Error, UpstreamConnectionError, UpstreamError

### Community 51 - "Vo"
Cohesion: 0.25
Nodes (4): UnifiedContent, UnifiedContentPart, UnifiedMessage, UnifiedToolCall

### Community 52 - "Session Management"
Cohesion: 0.25
Nodes (7): MessageDetailProjection, PageParam, SessionDetailProjection, SessionReadRepository, SessionRepository, SessionSummaryProjection, ToolDetailProjection

### Community 53 - "Goroutine Pool"
Cohesion: 0.36
Nodes (6): ComputeMessageChecksum(), ComputeToolChecksum(), jsonEqual(), normalizeArgumentsWithSchema(), toolChecksumWire, ToolSchemaMap

### Community 55 - "Aggregate Roots"
Cohesion: 0.29
Nodes (3): Tool, RecordTool(), RestoreTool()

### Community 59 - "Goroutine Pool"
Cohesion: 0.33
Nodes (3): InitPoolManager(), StopPoolManager(), PoolManager

### Community 61 - "Dto"
Cohesion: 0.33
Nodes (5): DetailedUser, GetCurUserRsp, UpdateUserReq, UpdateUserReqBody, User

### Community 62 - "Dto"
Cohesion: 0.33
Nodes (5): CallbackReq, CallbackReqBody, CallbackRsp, LoginReq, LoginResp

### Community 63 - "Application Layer"
Cohesion: 0.33
Nodes (3): GetCurrentUserHandler, GetCurrentUserQuery, UserView

### Community 64 - "Logging"
Cohesion: 0.33
Nodes (4): newCLSCore(), init(), WithFCtx(), RecoverMiddleware()

### Community 65 - "Common Utilities"
Cohesion: 0.33
Nodes (5): CommonParam, PageInfo, PageParam, QueryParam, SortParam

### Community 66 - "Application Layer"
Cohesion: 0.33
Nodes (3): APIKeyView, ListAPIKeysHandler, ListAPIKeysQuery

### Community 67 - "Data Access Objects"
Cohesion: 0.33
Nodes (5): CommonParam, PageInfo, PageParam, QueryParam, SortParam

### Community 75 - "Configuration"
Cohesion: 0.5
Nodes (4): init(), initEnvironment(), PoolConfig, PoolGroupConfig

### Community 76 - "Llmproxy"
Cohesion: 0.4
Nodes (4): EndpointAliasProjection, EndpointCredentialProjection, EndpointReadRepository, EndpointRepository

### Community 78 - "Dto"
Cohesion: 0.5
Nodes (3): CommonRsp, EmptyReq, EmptyRsp

### Community 83 - "Dto"
Cohesion: 0.5
Nodes (3): HTTPResponse, RedirectResponse, SSEResponse

### Community 86 - "Repository Layer"
Cohesion: 0.5
Nodes (3): GetModelCallAuditDAO(), NewAuditRepository(), auditRepository

## Knowledge Gaps
- **302 isolated node(s):** `User`, `DetailedUser`, `GetCurUserRsp`, `UpdateUserReq`, `UpdateUserReqBody` (+297 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **61 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `WithCtx()` connect `Logging` to `LLM Proxy Use Cases`, `Agent Pipeline`, `OAuth2`, `Aggregate Roots`, `HTTP Middleware`, `Transport Layer`, `Error Handling`, `OAuth2`, `LLM Proxy Use Cases`, `Application Layer`, `HTTP Middleware`, `Application Layer`, `Cron Jobs`, `Application Layer`, `Goroutine Pool`, `Cron Jobs`, `Application Layer`, `Logging`, `Application Layer`, `Cron Jobs`, `Application Layer`, `Cron Jobs`?**
  _High betweenness centrality (0.139) - this node is a cross-community bridge._
- **Why does `New()` connect `HTTP Middleware` to `LLM Proxy Use Cases`, `Repository Layer`, `Application Bootstrap`, `Aggregate Roots`, `Error Handling`, `OAuth2`, `Aggregate Roots`, `Aggregate Roots`, `Logging`, `Application Layer`, `HTTP Middleware`, `Application Layer`, `Cron Jobs`, `Application Layer`, `Aggregate Roots`, `Aggregate Roots`, `Aggregate Roots`, `Cron Jobs`, `Application Layer`, `Logging`, `Cron Jobs`, `Application Layer`, `JWT Auth`, `Configuration`, `Cron Jobs`?**
  _High betweenness centrality (0.115) - this node is a cross-community bridge._
- **Why does `Wrap()` connect `Repository Layer` to `LLM Proxy Use Cases`, `Application Layer`, `Protocol Converter`, `Error Handling`, `Protocol Converter`, `Transport Layer`, `OAuth2`, `Object Storage`, `Logging`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Are the 59 inferred relationships involving `WithCtx()` (e.g. with `.HandleGetCurUser()` and `.HandleUpdateUser()`) actually correct?**
  _`WithCtx()` has 59 INFERRED edges - model-reasoned connections that need verification._
- **Are the 51 inferred relationships involving `Wrap()` (e.g. with `FromOpenAIMessage()` and `FromAnthropicMessage()`) actually correct?**
  _`Wrap()` has 51 INFERRED edges - model-reasoned connections that need verification._
- **Are the 40 inferred relationships involving `New()` (e.g. with `convertOpenAIContentPart()` and `CORSMiddleware()`) actually correct?**
  _`New()` has 40 INFERRED edges - model-reasoned connections that need verification._
- **Are the 36 inferred relationships involving `GetDBInstance()` (e.g. with `JwtMiddleware()` and `APIKeyMiddleware()`) actually correct?**
  _`GetDBInstance()` has 36 INFERRED edges - model-reasoned connections that need verification._