package clickhouse

const (
	loggerName = "clickhouse"
)

type clickhouseConfig struct {
	Addr        []string `config:"addr"`
	Username    string   `config:"username"`
	Password    string   `config:"password"`
	Table       string   `config:"table"`
	Columns     []string `config:"columns"`
	MaxRetries  int      `config:"max_retries"`
	BulkMaxSize int      `config:"bulk_max_size"`
}

var (
	defaultConfig = clickhouseConfig{
		Addr:        []string{"127.0.0.1:9000"},
		MaxRetries:  3,
		BulkMaxSize: 1000,
	}
)
