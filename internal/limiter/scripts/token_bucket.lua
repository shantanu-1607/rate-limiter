local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])

local state = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(state[1])
local ts = tonumber(state[2])

if tokens == nil then
	tokens = capacity
  	ts = now
end

local elapsed = math.max(0,now - ts)
tokens = math.min(capacity, tokens+elapsed*rate)

local allowed = 0
if tokens >= cost then
	allowed = 1
	tokens = tokens - cost
end

redis.call('HMSET', KEYS[1], 'tokens', tokens, 'ts', now)
redis.call('EXPIRE', KEYS[1], math.ceil(capacity/rate)*2)

return {allowed, tostring(tokens)}