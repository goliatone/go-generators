package config

type Config struct {
	Logger   Logger
	Database *Database
}

type Logger struct {
	Level    string
	Filename string
}

type Database struct {
	DNS        string
	Debug      bool
	ClusterIPs []string
	Metadata   map[string]any
	Any        any
	NullTest   psql.Null
}
