package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HttpServer struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"http-server"`

	GrpcServer struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"grpc-server"`

	Products struct {
		Host   string `yaml:"host"`
		Port   string `yaml:"port"`
		Token  string `yaml:"token"`
		Schema string `yaml:"schema"`
	} `yaml:"products"`

	Database struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		Name     string `yaml:"name"`
	} `yaml:"database"`
}

func LoadConfig(filename string) (*Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	config := &Config{}
	if err := yaml.NewDecoder(f).Decode(config); err != nil {
		return nil, err
	}

	return config, nil
}
