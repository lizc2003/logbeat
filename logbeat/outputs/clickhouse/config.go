package clickhouse

import "time"

const (
	loggerName = "clickhouse"

	maxRetries  = 3
	backoffInit = 1 * time.Second
	backoffMax  = 60 * time.Second
)

type clickhouseConfig struct {
	Addr        []string `config:"addr"`
	Username    string   `config:"username"`
	Password    string   `config:"password"`
	Table       string   `config:"table"`
	Columns     []string `config:"columns"`
	BulkMaxSize int      `config:"bulk_max_size"`
}

var (
	defaultConfig = clickhouseConfig{
		Addr:        []string{"127.0.0.1:9000"},
		BulkMaxSize: 100,
	}
)
