package warp

import "strings"

func IsHTTPClientError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "API request failed with status: 5")
}
