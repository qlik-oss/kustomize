package utils

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"go.uber.org/zap"
)

const (
	DefaultLockRetryDelayMinMilliSeconds = 500
	DefaultLockRetryDelayMaxMilliSeconds = 1000
	DefaultLockTimeoutSeconds            = 60
)

var fileLockMutexMap = fileLockMutexMapT{
	mutexMap: make(map[string]*sync.Mutex),
}

type fileLockMutexMapT struct {
	mutexMapGuardMutex sync.Mutex
	mutexMap           map[string]*sync.Mutex
}

func (mm *fileLockMutexMapT) getMutexForKey(key string) *sync.Mutex {
	mm.mutexMapGuardMutex.Lock()
	defer mm.mutexMapGuardMutex.Unlock()

	var mutexForKey *sync.Mutex
	if mu, ok := mm.mutexMap[key]; ok {
		mutexForKey = mu
	} else {
		mutexForKey = new(sync.Mutex)
		mm.mutexMap[key] = mutexForKey
	}
	return mutexForKey
}

func LockPath(lockFilePath string, lockTimeoutSeconds, retryDelayMinMilliseconds, retryDelayMaxMilliseconds int, logger *zap.SugaredLogger) (unlockFn func(), err error) {
	t1 := time.Now()
	mu := fileLockMutexMap.getMutexForKey(lockFilePath)
	mu.Lock()

	defer func() {
		logger.Debugf("waited for file lock: %v for: %vs\n", lockFilePath, time.Now().Sub(t1).Seconds())
	}()
	if err := os.MkdirAll(filepath.Dir(lockFilePath), os.ModePerm); err != nil && !os.IsExist(err) {
		mu.Unlock()
		logger.Errorf("file lock failed for: %v, due to directory existance test, error: %v", lockFilePath, err)
		return nil, err
	}
	fileLock := flock.New(lockFilePath)
	lockCtx, cancel := context.WithTimeout(context.Background(), time.Duration(lockTimeoutSeconds)*time.Second)
	defer cancel()
	locked, err := tryLockContext(fileLock, lockCtx, retryDelayMinMilliseconds, retryDelayMaxMilliseconds)
	if err != nil {
		mu.Unlock()
		logger.Errorf("file lock failed for: %v with error: %v", lockFilePath, err)
		return nil, err
	} else if !locked {
		mu.Unlock()
		msg := fmt.Sprintf("file lock failed for: %v without an error", lockFilePath)
		logger.Errorf(msg)
		return nil, fmt.Errorf(msg)
	}

	t2 := time.Now()
	return func() {
		_ = fileLock.Unlock()
		_ = os.Remove(lockFilePath)
		mu.Unlock()
		logger.Debugf("releasing file lock: %v after: %vs\n", lockFilePath, time.Now().Sub(t2).Seconds())
	}, nil
}

func tryLockContext(fileLock *flock.Flock, ctx context.Context, retryDelayMinMilliseconds, retryDelayMaxMilliseconds int) (bool, error) {
	randomInt := func(min, max int) int {
		return min + rand.Intn(max-min)
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	for {
		if ok, err := fileLock.TryLock(); ok || err != nil {
			return ok, err
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(time.Duration(randomInt(retryDelayMinMilliseconds, retryDelayMaxMilliseconds)) * time.Millisecond):
			// try again
		}
	}
}
