package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const logRetentionInterval = 5 * time.Minute

var logRetentionTaskOnce sync.Once

func StartLogRetentionTask() {
	logRetentionTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}

		gopool.Go(func() {
			run := func() {
				deleted, err := model.TrimAllUserLogsBeyondLimit(context.Background(), model.UserLogRetentionLimit)
				if err != nil {
					logger.LogWarn(context.Background(), fmt.Sprintf("log retention cleanup failed: %v", err))
					return
				}
				if deleted > 0 {
					logger.LogInfo(context.Background(), fmt.Sprintf("log retention cleanup removed %d rows", deleted))
				}
			}

			run()
			ticker := time.NewTicker(logRetentionInterval)
			defer ticker.Stop()
			for range ticker.C {
				run()
			}
		})
	})
}
