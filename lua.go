package redis_rate

import "github.com/go-redis/redis/v7"

// Copyright (c) 2017 Pavel Pravosud
// https://github.com/rwz/redis-gcra/blob/master/vendor/perform_gcra_ratelimit.lua
var gcra = redis.NewScript(`
-- this script has side-effects, so it requires replicate commands mode
redis.replicate_commands()

local rate_limit_key = KEYS[1]
local burst = ARGV[1]
local rate = ARGV[2]
local period = ARGV[3]
local cost = ARGV[4]

local emission_interval = period / rate
local increment = emission_interval * cost
local burst_offset = emission_interval * burst
local now = redis.call("TIME")

-- redis returns time as an array containing two integers: seconds of the epoch
-- time (10 digits) and microseconds (6 digits). for convenience we need to
-- convert them to a floating point number. the resulting number is 16 digits,
-- bordering on the limits of a 64-bit double-precision floating point number.
-- adjust the epoch to be relative to Jan 1, 2017 00:00:00 GMT to avoid floating
-- point problems. this approach is good until "now" is 2,483,228,799 (Wed, 09
-- Sep 2048 01:46:39 GMT), when the adjusted value is 16 digits.
local jan_1_2017 = 1483228800
now = (now[1] - jan_1_2017) + (now[2] / 1000000)

local tat = redis.call("GET", rate_limit_key)

if not tat then
  tat = now
else
  tat = tonumber(tat)
end

local new_tat = math.max(tat, now) + increment

local allow_at = new_tat - burst_offset
local diff = now - allow_at

local limited
local retry_after
local reset_after

local remaining = math.floor(diff / emission_interval + 0.5) -- poor man's round

if remaining < 0 then
  limited = 1
  remaining = 0
  reset_after = tat - now
  retry_after = diff * -1
else
  limited = 0
  reset_after = new_tat - now
  redis.call("SET", rate_limit_key, new_tat, "EX", math.ceil(reset_after))
  retry_after = -1
end

return {limited, remaining, tostring(retry_after), tostring(reset_after)}
`)
