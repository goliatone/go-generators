package basic

type Option func(*Config)

func WithName(name string) Option {
	return func(c *Config) {
		c.name = name
	}
}

func WithValue(val int) Option {
	return func(c *Config) {
		c.value = val
	}
}
