// Copyright 2014 beego Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package memcache for cache provider
//
// depend on github.com/bradfitz/gomemcache/memcache
//
// go install github.com/bradfitz/gomemcache/memcache
//
// Usage:
// import(
//
//	_ "github.com/bcjti/cache/memcache"
//	"github.com/bcjti/cache"
//
// )
//
//	bm, err := cache.NewCache("memcache", `{"conn":"127.0.0.1:11211"}`)
//
//	more docs http://beego.me/docs/module/cache.md
package memcache

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/bcjti/cache"
	"github.com/bradfitz/gomemcache/memcache"
)

// Cache Memcache adapter.
type Cache struct {
	conn     *memcache.Client
	conninfo []string
}

// NewMemCache create new memcache adapter.
func NewMemCache() cache.Cache {
	return &Cache{}
}

// Get get value from memcache.
func (rc *Cache) Get(key string) interface{} {
	if rc.conn == nil {
		if err := rc.connectInit(); err != nil {
			return err
		}
	}
	if item, err := rc.conn.Get(key); err == nil {
		return item.Value
	}
	return nil
}

// GetMulti get value from memcache.
func (rc *Cache) GetMulti(keys []string) []interface{} {
	size := len(keys)
	var rv []interface{}
	if rc.conn == nil {
		if err := rc.connectInit(); err != nil {
			for i := 0; i < size; i++ {
				rv = append(rv, err)
			}
			return rv
		}
	}
	mv, err := rc.conn.GetMulti(keys)
	if err == nil {
		for _, v := range mv {
			rv = append(rv, v.Value)
		}
		return rv
	}
	for i := 0; i < size; i++ {
		rv = append(rv, err)
	}
	return rv
}

// Put put value to memcache.
func (rc *Cache) Put(key string, val interface{}, timeout time.Duration) error {
	if rc.conn == nil {
		if err := rc.connectInit(); err != nil {
			return err
		}
	}
	item := memcache.Item{Key: key, Expiration: int32(timeout / time.Second)}
	if v, ok := val.([]byte); ok {
		item.Value = v
	} else if str, ok := val.(string); ok {
		item.Value = []byte(str)
	} else {
		return errors.New("val only support string and []byte")
	}
	return rc.conn.Set(&item)
}

// Delete delete value in memcache.
func (rc *Cache) Delete(key string) error {
	if rc.conn == nil {
		if err := rc.connectInit(); err != nil {
			return err
		}
	}
	return rc.conn.Delete(key)
}

// Incr increases counter by 1.
// it is equivalent to IncrBy(key, 1), discarding the new value.
func (rc *Cache) Incr(key string) error {
	_, err := rc.IncrBy(key, 1)
	return err
}

// IncrBy increases the counter by increment and returns the new value as
// int64. increment must be >= 0 (use DecrBy to decrease).
// the stored value must be an ASCII decimal string (seed it with
// Put(key, "0", ttl)); a missing key is created as 0, without expiration,
// before the increment is applied.
func (rc *Cache) IncrBy(key string, increment int) (int64, error) {
	if increment < 0 {
		return 0, errors.New("increment must be >= 0, use DecrBy to decrease")
	}
	return rc.incrDecr(key, uint64(increment), false)
}

// Decr decreases counter by 1.
// it is equivalent to DecrBy(key, 1), discarding the new value.
func (rc *Cache) Decr(key string) error {
	_, err := rc.DecrBy(key, 1)
	return err
}

// DecrBy decreases the counter by decrement and returns the new value as
// int64. decrement must be >= 0 (use IncrBy to increase). memcached counters
// are unsigned: decreasing below 0 clamps the value at 0 without error.
// see IncrBy for the stored-value and missing-key semantics.
func (rc *Cache) DecrBy(key string, decrement int) (int64, error) {
	if decrement < 0 {
		return 0, errors.New("decrement must be >= 0, use IncrBy to increase")
	}
	return rc.incrDecr(key, uint64(decrement), true)
}

// incrDecr runs memcached incr/decr with delta, creating a missing key as
// "0" first, and returns the new value converted to int64.
func (rc *Cache) incrDecr(key string, delta uint64, negate bool) (int64, error) {
	if rc.conn == nil {
		if err := rc.connectInit(); err != nil {
			return 0, err
		}
	}
	op := rc.conn.Increment
	if negate {
		op = rc.conn.Decrement
	}
	nv, err := op(key, delta)
	if err == memcache.ErrCacheMiss {
		// memcached never auto-creates counters: seed the key with "0" and
		// retry once. losing the Add race (ErrNotStored) is fine — the
		// retry applies the delta to the concurrent writer's value.
		if err = rc.conn.Add(&memcache.Item{Key: key, Value: []byte("0")}); err != nil && err != memcache.ErrNotStored {
			return 0, err
		}
		nv, err = op(key, delta)
	}
	if err != nil {
		return 0, err
	}
	if nv > math.MaxInt64 {
		return 0, errors.New("value overflows int64")
	}
	return int64(nv), nil
}

// IsExist check value exists in memcache.
func (rc *Cache) IsExist(key string) bool {
	if rc.conn == nil {
		if err := rc.connectInit(); err != nil {
			return false
		}
	}
	_, err := rc.conn.Get(key)
	return err == nil
}

// ClearAll clear all cached in memcache.
func (rc *Cache) ClearAll() error {
	if rc.conn == nil {
		if err := rc.connectInit(); err != nil {
			return err
		}
	}
	return rc.conn.FlushAll()
}

// StartAndGC start memcache adapter.
// config string is like {"conn":"connection info"}.
// if connecting error, return.
func (rc *Cache) StartAndGC(config string) error {
	var cf map[string]string
	json.Unmarshal([]byte(config), &cf)
	if _, ok := cf["conn"]; !ok {
		return errors.New("config has no conn key")
	}
	rc.conninfo = strings.Split(cf["conn"], ";")
	if rc.conn == nil {
		if err := rc.connectInit(); err != nil {
			return err
		}
	}
	return nil
}

// connect to memcache and keep the connection.
func (rc *Cache) connectInit() error {
	rc.conn = memcache.New(rc.conninfo...)
	return nil
}

func init() {
	cache.Register("memcache", NewMemCache)
}
