package tools

import (
	"context"
	"errors"
	"sync"
	"time"
)

var GlmLock *KeyLocker

type KeyLocker struct {
	mu    sync.Mutex
	locks map[string]chan struct{}
	limit map[string]int
}

// 初始化
func NewKeyLocker(limit map[string]int) *KeyLocker {
	return &KeyLocker{
		locks: make(map[string]chan struct{}),
		limit: limit,
	}
}

func NewGlmLock(keys []string) *KeyLocker {
	limit := make(map[string]int)
	modelList := map[string]int{
		"GLM-4.6":                    3,
		"GLM-4.6V-FlashX":            3,
		"GLM-4.7":                    2,
		"GLM-Image":                  1,
		"GLM-5-Turbo":                1,
		"GLM-5V-Turbo":               1,
		"GLM-5.1":                    1,
		"GLM-4.5":                    10,
		"GLM-4.6V":                   10,
		"GLM-4.7-Flash":              1,
		"GLM-4.7-FlashX":             3,
		"GLM-OCR":                    2,
		"GLM-5":                      2,
		"GLM-4-Plus":                 20,
		"GLM-4.5V":                   10,
		"GLM-4.6V-Flash":             1,
		"AutoGLM-Phone-Multilingual": 5,
		"GLM-4.5-Air":                5,
		"GLM-4.5-AirX":               5,
		"GLM-4.5-Flash":              2,
		"GLM-4-32B-0414-128K":        15,
		"CogView-4-250304":           5,
		"GLM-ASR-2512":               5,
		"ViduQ1-text":                5,
		"Viduq1-Image":               5,
		"Viduq1-Start-End":           5,
		"Vidu2-Image":                5,
		"Vidu2-Start-End":            5,
		"Vidu2-Reference":            5,
		"CogVideoX-3":                1,
	}

	for _, key := range keys {
		for modelName, modelLimit := range modelList {
			limit[key+modelName] = modelLimit
		}
	}
	return NewKeyLocker(limit)
}

// 获取锁（带超时）
func (k *KeyLocker) Acquire(key string, timeout time.Duration) error {
	k.mu.Lock()
	ch, ok := k.locks[key]
	if !ok {
		// 初始化该 key 的并发池
		size := k.limit[key]
		if size <= 0 {
			size = 1 // 默认1
		}
		ch = make(chan struct{}, size)
		k.locks[key] = ch
	}
	k.mu.Unlock()

	// 带超时的获取
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return errors.New("获取锁超时")
	}
}

// 释放锁
func (k *KeyLocker) Release(key string) {
	k.mu.Lock()
	ch, ok := k.locks[key]
	k.mu.Unlock()

	if !ok {
		return
	}

	select {
	case <-ch:
	default:
		// 防止多释放
	}
}
