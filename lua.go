package redis_rate //nolint:revive // upstream used this name

import (
	_ "embed"

	"github.com/redis/go-redis/v9"
)

// Copyright (c) 2017 Pavel Pravosud
// https://github.com/rwz/redis-gcra/blob/master/vendor/perform_gcra_ratelimit.lua

//go:embed script_allow_n.lua
var alloNScript string

//go:embed script_allow_at_most.lua
var allowAtMostScript string

var allowN = redis.NewScript(alloNScript)

var allowAtMost = redis.NewScript(allowAtMostScript)
