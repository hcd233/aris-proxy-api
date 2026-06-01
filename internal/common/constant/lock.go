package constant

const (
	// LuaRefreshLock 仅持有者可以 PEXPIRE 的 Lua 脚本
	LuaRefreshLock = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
    return 0
end
`

	// LuaUnlockLock 仅持有者可以 DEL 的 Lua 脚本
	LuaUnlockLock = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`
)
