package boot

import (
	"altalune.id/othra/config"
	"altalune.id/othra/internal/kernel/queue"
)

func consumerOff(o *options, cfg config.Config) string {
	switch {
	case !o.consumer:
		return "WithConsumer(false)"
	case o.schedulerOnly:
		return "WithSchedulerOnly"
	case !cfg.Queue.Enabled:
		return "queue.enabled=false"
	}
	return ""
}

func queueHandlers(_ *Services) []queue.Handler { return nil }

func queueListeners(_ *Services) []queue.Listener { return nil }
