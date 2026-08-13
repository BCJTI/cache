package ssdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bcjti/cache"
	"github.com/ssdb/gossdb/ssdb"
)

// DefaultTimeout bounds each request/response round-trip when the config
// carries no "timeout" entry.
var DefaultTimeout = 60 * time.Second

// Cache SSDB adapter
type Cache struct {
	lock     sync.Mutex // gossdb's Client is not goroutine safe: serialize every round-trip
	conn     *ssdb.Client
	conninfo []string
	timeout  time.Duration
}

// NewSsdbCache create new ssdb adapter.
func NewSsdbCache() cache.Cache {
	return &Cache{}
}

// do serializes one request/response round-trip on the shared gossdb
// connection, lazily connecting and dropping the connection on a transport
// error (or malformed response) so the next call reconnects.
// gossdb offers no I/O deadlines, so a watchdog closes the socket if the
// round-trip exceeds the configured timeout — otherwise a hung server would
// block this call forever while it holds the adapter-wide lock.
func (rc *Cache) do(args ...interface{}) ([]string, error) {
	rc.lock.Lock()
	defer rc.lock.Unlock()
	if rc.conn == nil {
		if err := rc.connectInit(); err != nil {
			return nil, err
		}
	}
	timeout := rc.timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	conn := rc.conn
	watchdog := time.AfterFunc(timeout, func() { conn.Close() })
	resp, err := rc.conn.Do(args...)
	watchdog.Stop()
	if err != nil || resp == nil {
		rc.conn.Close()
		rc.conn = nil
		if err == nil {
			err = errors.New("ssdb: malformed response")
		}
		return nil, err
	}
	return resp, nil
}

// Get get value from ssdb.
func (rc *Cache) Get(key string) interface{} {
	resp, err := rc.do("get", key)
	if err != nil {
		return nil
	}
	if len(resp) == 2 && resp[0] == "ok" {
		return resp[1]
	}
	return nil
}

// GetMulti get values from ssdb.
func (rc *Cache) GetMulti(keys []string) []interface{} {
	size := len(keys)
	var values []interface{}
	res, err := rc.do("multi_get", keys)
	if err == nil {
		resSize := len(res)
		for i := 1; i < resSize; i += 2 {
			values = append(values, res[i+1])
		}
		return values
	}
	for i := 0; i < size; i++ {
		values = append(values, err)
	}
	return values
}

// DelMulti delete values in ssdb.
func (rc *Cache) DelMulti(keys []string) error {
	_, err := rc.do("multi_del", keys)
	return err
}

// Put put value to ssdb. only support string.
func (rc *Cache) Put(key string, value interface{}, timeout time.Duration) error {
	v, ok := value.(string)
	if !ok {
		return errors.New("value must string")
	}
	var resp []string
	var err error
	ttl := int(timeout / time.Second)
	if ttl < 0 {
		resp, err = rc.do("set", key, v)
	} else {
		resp, err = rc.do("setx", key, v, ttl)
	}
	if err != nil {
		return err
	}
	if len(resp) == 2 && resp[0] == "ok" {
		return nil
	}
	return errors.New("bad response")
}

// Delete delete value in ssdb.
func (rc *Cache) Delete(key string) error {
	resp, err := rc.do("del", key)
	if err != nil {
		return err
	}
	if len(resp) > 0 && resp[0] == "ok" {
		return nil
	}
	return fmt.Errorf("ssdb: bad response %v", resp)
}

// Incr increases counter by 1.
// it is equivalent to IncrBy(key, 1), discarding the new value.
func (rc *Cache) Incr(key string) error {
	_, err := rc.IncrBy(key, 1)
	return err
}

// IncrBy increases the counter by increment and returns the new value as
// int64 (ssdb incr reply). increment must be >= 0 (use DecrBy to decrease).
// a missing key is created as 0 before the increment is applied.
func (rc *Cache) IncrBy(key string, increment int) (int64, error) {
	if increment < 0 {
		return 0, errors.New("increment must be >= 0, use DecrBy to decrease")
	}
	return rc.incrBy(key, increment)
}

// Decr decreases counter by 1.
// it is equivalent to DecrBy(key, 1), discarding the new value.
func (rc *Cache) Decr(key string) error {
	_, err := rc.DecrBy(key, 1)
	return err
}

// DecrBy decreases the counter by decrement and returns the new value as
// int64 (may be negative). decrement must be >= 0 (use IncrBy to increase).
// a missing key is created as 0 before the decrement is applied.
func (rc *Cache) DecrBy(key string, decrement int) (int64, error) {
	if decrement < 0 {
		return 0, errors.New("decrement must be >= 0, use IncrBy to increase")
	}
	return rc.incrBy(key, -decrement)
}

// incrBy runs ssdb's incr command (used with a negative n to decrease; ssdb
// KV has no separate decr), validates the response status — gossdb only
// reports transport errors, so server-side failures such as incrementing a
// non-numeric value surface here — and returns the new value.
func (rc *Cache) incrBy(key string, n int) (int64, error) {
	resp, err := rc.do("incr", key, n)
	if err != nil {
		return 0, err
	}
	if len(resp) == 2 && resp[0] == "ok" {
		return strconv.ParseInt(resp[1], 10, 64)
	}
	return 0, fmt.Errorf("ssdb: bad response %v", resp)
}

// IsExist check value exists in ssdb.
func (rc *Cache) IsExist(key string) bool {
	resp, err := rc.do("exists", key)
	if err != nil {
		return false
	}
	if len(resp) == 2 && resp[1] == "1" {
		return true
	}
	return false
}

// ClearAll clear all cached in ssdb.
func (rc *Cache) ClearAll() error {
	keyStart, keyEnd, limit := "", "", 50
	resp, err := rc.Scan(keyStart, keyEnd, limit)
	for err == nil {
		size := len(resp)
		if size == 1 {
			return nil
		}
		keys := []string{}
		for i := 1; i < size; i += 2 {
			keys = append(keys, resp[i])
		}
		_, e := rc.do("multi_del", keys)
		if e != nil {
			return e
		}
		keyStart = resp[size-2]
		resp, err = rc.Scan(keyStart, keyEnd, limit)
	}
	return err
}

// Scan key all cached in ssdb.
func (rc *Cache) Scan(keyStart string, keyEnd string, limit int) ([]string, error) {
	resp, err := rc.do("scan", keyStart, keyEnd, limit)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// StartAndGC start ssdb adapter.
// config string is like {"conn":"connection info"} with an optional
// "timeout" duration (e.g. "30s") bounding each round-trip.
// if connecting error, return.
func (rc *Cache) StartAndGC(config string) error {
	var cf map[string]string
	json.Unmarshal([]byte(config), &cf)
	if _, ok := cf["conn"]; !ok {
		return errors.New("config has no conn key")
	}
	rc.lock.Lock()
	defer rc.lock.Unlock()
	rc.conninfo = strings.Split(cf["conn"], ";")
	rc.timeout = DefaultTimeout
	if v, ok := cf["timeout"]; ok {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return fmt.Errorf("ssdb: invalid timeout %q", v)
		}
		rc.timeout = d
	}
	if rc.conn == nil {
		if err := rc.connectInit(); err != nil {
			return err
		}
	}
	return nil
}

// connect to ssdb and keep the connection. the caller must hold rc.lock.
func (rc *Cache) connectInit() error {
	if len(rc.conninfo) == 0 {
		return errors.New("ssdb: not configured, call StartAndGC first")
	}
	parts := strings.Split(rc.conninfo[0], ":")
	if len(parts) != 2 {
		return fmt.Errorf("ssdb: invalid conn %q", rc.conninfo[0])
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("ssdb: invalid port in conn %q: %v", rc.conninfo[0], err)
	}
	rc.conn, err = ssdb.Connect(parts[0], port)
	return err
}

func init() {
	cache.Register("ssdb", NewSsdbCache)
}
