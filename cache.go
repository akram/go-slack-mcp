package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type userCache struct {
	mu       sync.RWMutex
	entries  map[string]string
	filePath string
}

func newUserCache(dir string) *userCache {
	uc := &userCache{
		entries:  make(map[string]string),
		filePath: filepath.Join(dir, ".user_cache.json"),
	}
	uc.load()
	return uc
}

func (uc *userCache) load() {
	data, err := os.ReadFile(uc.filePath)
	if err != nil {
		return
	}
	uc.mu.Lock()
	defer uc.mu.Unlock()
	_ = json.Unmarshal(data, &uc.entries)
	logStderr("Loaded %d user handles from cache", len(uc.entries))
}

func (uc *userCache) save() {
	uc.mu.RLock()
	data, err := json.MarshalIndent(uc.entries, "", "  ")
	uc.mu.RUnlock()
	if err != nil {
		return
	}
	_ = os.WriteFile(uc.filePath, data, 0o644)
}

func (uc *userCache) get(userID string) (string, bool) {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	v, ok := uc.entries[userID]
	return v, ok
}

func (uc *userCache) set(userID, handle string) {
	uc.mu.Lock()
	uc.entries[userID] = handle
	uc.mu.Unlock()
	uc.save()
}

func (uc *userCache) clear() int {
	uc.mu.Lock()
	count := len(uc.entries)
	uc.entries = make(map[string]string)
	uc.mu.Unlock()
	_ = os.Remove(uc.filePath)
	logStderr("Cleared %d user cache entries", count)
	return count
}

type channelCache struct {
	mu      sync.RWMutex
	entries map[string]string // name -> id
}

func newChannelCache() *channelCache {
	return &channelCache{entries: make(map[string]string)}
}

func (cc *channelCache) get(name string) (string, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	v, ok := cc.entries[name]
	return v, ok
}

func (cc *channelCache) set(name, id string) {
	cc.mu.Lock()
	cc.entries[name] = id
	cc.mu.Unlock()
}

func (cc *channelCache) clear() {
	cc.mu.Lock()
	cc.entries = make(map[string]string)
	cc.mu.Unlock()
}

func (cc *channelCache) size() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return len(cc.entries)
}

func logStderr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
