// 定义 generateMessageID 生成消息ID
package utils

import (
	"fmt"
	"time"
)

func GenerateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
