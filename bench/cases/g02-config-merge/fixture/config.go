package config

// Config is the worker configuration after all layers are applied.
type Config struct {
	Debug   bool
	Port    int
	Region  string
	Retries int
}

// Defaults is the lowest layer.
var Defaults = Config{Debug: false, Port: 8080, Region: "us-east-1", Retries: 3}

// Merge applies override on top of base.
func Merge(base, override Config) Config {
	out := base
	if override.Debug {
		out.Debug = true
	}
	if override.Port != 0 {
		out.Port = override.Port
	}
	if override.Region != "" {
		out.Region = override.Region
	}
	if override.Retries != 0 {
		out.Retries = override.Retries
	}
	return out
}

// Load applies the file layer and then the environment layer.
func Load(file, env Config) Config {
	return Merge(Merge(Defaults, file), env)
}
