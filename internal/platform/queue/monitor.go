package queue

import (
	"net/http"

	"github.com/hibiken/asynqmon"
)

func MonitorHandler(url string) (http.Handler, error) {
	opt, err := RedisOpt(url)
	if err != nil {
		return nil, err
	}
	return asynqmon.New(asynqmon.Options{
		RootPath:     "/admin/queue",
		RedisConnOpt: opt,
	}), nil
}
