package queue

import (
	"fmt"

	"github.com/hibiken/asynq"
)

func NewScheduler(url string) (*asynq.Scheduler, error) {
	opt, err := RedisOpt(url)
	if err != nil {
		return nil, err
	}
	sched := asynq.NewScheduler(opt, nil)
	task := asynq.NewTask(TaskFraudScan, nil, asynq.Queue(QueueDefault), asynq.MaxRetry(2))
	if _, err := sched.Register("0 0 * * *", task); err != nil {
		return nil, fmt.Errorf("queue: register fraud cron: %w", err)
	}
	return sched, nil
}
