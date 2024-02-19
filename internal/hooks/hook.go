package hooks

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const BaseHash = "0000000000000000000000000000000000000000"

func pushOptions() map[string]string {
	opts := make(map[string]string)
	if pushCount, err := strconv.Atoi(os.Getenv("GIT_PUSH_OPTION_COUNT")); err == nil {
		for i := 0; i < pushCount; i++ {
			opt := os.Getenv(fmt.Sprintf("GIT_PUSH_OPTION_%d", i))
			kv := strings.SplitN(opt, "=", 2)
			if len(kv) == 2 {
				opts[kv[0]] = kv[1]
			}
		}
	}
	return opts
}
