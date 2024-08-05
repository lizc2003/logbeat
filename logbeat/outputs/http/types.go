package http

import (
	"encoding/json"
)

const (
	originMsgKey = "#originMsg"
)

type eventRaw map[string]json.RawMessage
