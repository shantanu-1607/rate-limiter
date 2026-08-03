-- KEYS[1] = bucket key (e.g. "rl:sw:pro")
-- ARGV[1] = capacity / limit (max requests allowed in window, e.g. 100)
-- ARGV[2] = window size in seconds (e.g. 60)
-- ARGV[3] = now (current epoch timestamp in seconds as float)
-- ARGV[4] = request_id (unique identifier for this request)
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local request_id = ARGV[4]
local clear_before = now - window
-- 1. Remove all request entries older than current window
redis.call('ZREMRANGEBYSCORE', key, '-inf', clear_before)
-- 2. Count remaining valid entries in current window
local current_count = redis.call('ZCARD', key)
if current_count < limit then
    -- 3. Add current request timestamp to sorted set
    redis.call('ZADD', key, now, request_id)
    -- 4. Refresh key expiration so Redis cleans up inactive keys
    redis.call('EXPIRE', key, math.ceil(window))
    
    local remaining = limit - current_count - 1
    return {1, tostring(remaining)}
else
    return {0, "0.0"}
end