## cache
cache is a Go cache manager. It can use many cache adapters. The repo is inspired by `database/sql` .


## How to install?

	go get github.com/bcjti/cache


## What adapters are supported?

As of now this cache support memory, Memcache and Redis.


## How to use it?

First you must import it

	import (
		"github.com/bcjti/cache"
	)

Then init a Cache (example with memory adapter)

	bm, err := cache.NewCache("memory", `{"interval":60}`)	

Use it like this:	
	
	bm.Put("astaxie", 1, 10 * time.Second)
	bm.Get("astaxie")
	bm.IsExist("astaxie")
	bm.Delete("astaxie")

Sets, expiration and conditional writes:

	added, err := bm.SAdd("tags", "a", "b")    // atomically add members, returns how many were new
	removed, err := bm.SRem("tags", "a")       // atomically remove members; an emptied set is deleted
	members, err := bm.SMembers("tags")        // all members, in no particular order
	ok, err := bm.Expire("tags", time.Hour)    // new TTL for an existing key (false if missing)
	ok, err = bm.PutIfExists("k", "v", ttl)    // write only if the key exists (false if missing)

Support per adapter:

| Operation             | memory | file  | redis                  | memcache | ssdb   |
|-----------------------|--------|-------|------------------------|----------|--------|
| SAdd, SRem, SMembers  | yes    | yes*  | yes (SADD/SREM/SMEMBERS) | no     | no     |
| Expire                | yes    | yes*  | yes (PEXPIRE)          | yes (touch) | yes (expire) |
| PutIfExists           | yes    | yes*  | yes (SET XX)           | yes (replace) | no |

Unsupported operations return `cache.ErrNotSupported`.
\* the file adapter serializes these writes within one process only.


## Memory adapter

Configure memory adapter like this:

	{"interval":60}

interval means the gc time. The cache will check at each time interval, whether item has expired.


## Memcache adapter

Memcache adapter use the [gomemcache](http://github.com/bradfitz/gomemcache) client.

Configure like this:

	{"conn":"127.0.0.1:11211"}


## Redis adapter

Redis adapter use the [redigo](http://github.com/gomodule/redigo) client.

Configure like this:

	{"conn":":6039"}
