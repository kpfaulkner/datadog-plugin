package main

import (
  "sort"
  "time"
)

/*
type TimeCountPair struct {
  timestamp time.Time
  count int64
} */

type CacheEntry struct {
	Query     string
	StartTime time.Time
	EndTime   time.Time
	Data      map[time.Time]int64

}

func NewCacheEntry() *CacheEntry {
	ce := CacheEntry{}
	ce.Data = make(map[time.Time]int64)

	//ce.Data = []TimeCountPair{}
	return &ce
}

func (ce *CacheEntry) PruneBefore(t time.Time) (int,error) {

  pruneCount := 0
  for k,_ := range ce.Data {
    if k.Before(t) {
      delete(ce.Data,k)
      pruneCount += 1
    }
  }

	return pruneCount,nil
}

func (ce *CacheEntry) Clear() {
	ce.Data = make(map[time.Time]int64)
}

// Add new entry. replace existing if it already exists.
func (ce *CacheEntry) AddEntry(timeStamp time.Time, count int64,) error {
  ce.Data[timeStamp] = count
	return nil
}

func (ce *CacheEntry) PruneOld(oldestTimeStamp time.Time) error {

  // delete old one.
  for k, _ := range ce.Data {
    if k.Before(oldestTimeStamp) {
      delete(ce.Data, k)
    }
  }
  return nil
}

func (ce *CacheEntry) GetKeysInOrder() []time.Time {
  keys := make([]time.Time, len(ce.Data))
  i := 0
  for k := range ce.Data {
    keys[i] = k
    i++
  }

  sort.Slice(keys, func (i int, j int) bool {
    return keys[i].Before(keys[j])
  })

  return keys
}

type Cache interface {

	// Get from Cache based on query string and time
	// This might be too fine grain and the cache will take a hammering per query.
	// But will see.
	Get(query string) (CacheEntry, error)

	// Set the cache.
	Set(query string, cacheEntry CacheEntry) error
}

// SimpleCache.... what could possibly go wrong?!?!?
// Will replace with something "real" if a simplistic approach doesn't work.
type SimpleCache struct {

	// cache. Outer map takes query as key... then inside that is a key/value of time => counts.
	cache map[string]CacheEntry
}

func NewSimpleCache() *SimpleCache {
	sc := SimpleCache{}
	sc.cache = make(map[string]CacheEntry)
	return &sc
}

func (c *SimpleCache) Set(query string, ce *CacheEntry) error {

	// replacing existing entry (or add new)
	c.cache[query] = *ce

	return nil
}

// Get entry from cache.
// Uses similar pattern to map, second return value is bool indicating if
// value is good or not.
func (c *SimpleCache) Get(query string) (*CacheEntry, bool) {

	ce, ok := c.cache[query]
	if ok {
		return &ce, true
	}

	// if one doesn't exist, make a new CacheEntry and return that?
	// probably a BAD idea :)
	//nce := NewCacheEntry()

	return nil, false
}
