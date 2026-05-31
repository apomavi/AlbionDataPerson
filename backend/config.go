package backend

import "flag"

type Config struct {
	Addr           string
	CollectorToken string
	DatabaseURL    string
}

func ParseConfig() Config {
	cfg := Config{}
	flag.StringVar(&cfg.Addr, "addr", ":8082", "HTTP address for the standalone backend")
	flag.StringVar(&cfg.DatabaseURL, "db", "host=localhost port=5432 user=postgres password=5447495353aA. dbname=postgres sslmode=disable", "PostgreSQL connection string")
	flag.StringVar(&cfg.CollectorToken, "collector-token", "", "Optional bearer token required by the collector ingest API")
	flag.Parse()
	return cfg
}
