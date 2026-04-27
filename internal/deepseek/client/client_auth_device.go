package client

import (
	"strings"

	"ds2api/internal/config"
)

const defaultLoginDeviceID = "deepseek_to_api"

func loginDeviceID(acc config.Account) string {
	if deviceID := strings.TrimSpace(acc.DeviceID); deviceID != "" {
		return deviceID
	}
	return defaultLoginDeviceID
}
