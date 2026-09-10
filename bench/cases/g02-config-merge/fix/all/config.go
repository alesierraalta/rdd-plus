package config

// Config is the worker configuration after all layers are applied; a nil Debug or Retries means the layer did not set it.
type Config struct {
	Debug   *bool
	Port    int
	Region  string
	Retries *int
}

func boolp(v bool) *bool { return &v }
func intp(v int) *int    { return &v }

// Defaults is the lowest layer.
var Defaults = Config{Debug: boolp(false), Port: 8080, Region: "us-east-1", Retries: intp(3)}

// Merge applies override on top of base; a set field wins even when its value is false or 0.
func Merge(base, override Config) Config {
	out := base
	if override.Debug != nil {
		out.Debug = override.Debug
	}
	if override.Port != 0 {
		out.Port = override.Port
	}
	if override.Region != "" {
		out.Region = override.Region
	}
	if override.Retries != nil {
		out.Retries = override.Retries
	}
	return out
}

// Load applies the file layer and then the environment layer.
func Load(file, env Config) Config {
	return Merge(Merge(Defaults, file), env)
}
