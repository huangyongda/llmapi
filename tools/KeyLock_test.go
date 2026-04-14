package tools

import (
	"fmt"
	"testing"
	"time"
)

func TestKeyLock(t *testing.T) {
	locker := NewKeyLocker(map[string]int{
		"key1": 1, // 最大并发 2
		"key2": 1, // 最大并发 1
	})

	for i := 0; i < 5; i++ {
		go func(i int) {
			fmt.Println("goroutine", i, "开始获取锁", time.Now().Format("2006-01-02 15:04:05"))

			err := locker.Acquire("key1", 5*time.Second)
			if err != nil {
				fmt.Println("goroutine", i, "获取锁失败:", err, time.Now().Format("2006-01-02 15:04:05"))
				return
			}

			fmt.Println("goroutine", i, "获取锁成功", time.Now().Format("2006-01-02 15:04:05"))

			time.Sleep(4 * time.Second)

			locker.Release("key1")
			fmt.Println("goroutine", i, "释放锁", time.Now().Format("2006-01-02 15:04:05"))

		}(i)
	}

	time.Sleep(20 * time.Second)
}

func TestKeyLock2(t *testing.T) {
	keyName := "mykeys1"
	locker := NewGlmLock([]string{
		keyName,
	})

	for i := 0; i < 5; i++ {
		go func(i int) {
			modelName := "GLM-5.1"
			// fmt.Println(modelName, i, "开始获取锁", time.Now().Format("2006-01-02 15:04:05"))

			err := locker.Acquire(keyName+modelName, 5*time.Second)
			if err != nil {
				fmt.Println(modelName, i, "获取锁失败:", err, time.Now().Format("2006-01-02 15:04:05"))
				return
			}

			fmt.Println(modelName, i, "获取锁成功", time.Now().Format("2006-01-02 15:04:05"))

			time.Sleep(4 * time.Second)

			locker.Release(keyName + modelName)
			fmt.Println(modelName, i, "释放锁", time.Now().Format("2006-01-02 15:04:05"))

		}(i)
	}
	for i := 0; i < 5; i++ {
		go func(i int) {
			modelName := "GLM-4.7"
			// fmt.Println(modelName, i, "开始获取锁", time.Now().Format("2006-01-02 15:04:05"))

			err := locker.Acquire(keyName+modelName, 5*time.Second)
			if err != nil {
				fmt.Println(modelName, i, "获取锁失败:", err, time.Now().Format("2006-01-02 15:04:05"))
				return
			}

			fmt.Println(modelName, i, "获取锁成功", time.Now().Format("2006-01-02 15:04:05"))

			time.Sleep(4 * time.Second)

			locker.Release(keyName + modelName)
			fmt.Println(modelName, i, "释放锁", time.Now().Format("2006-01-02 15:04:05"))

		}(i)
	}

	time.Sleep(20 * time.Second)
}
