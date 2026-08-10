package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var config Config

type Config struct {
	Services    map[string]string
	GithubToken string
}

func loadConfig() error {
	config = Config{
		GithubToken: os.Getenv("GITHUB_TOKEN"),
		Services:    map[string]string{},
	}

	required := map[string]*string{
		"Github Token": &config.GithubToken,
	}

	var missing []string
	for name, value := range required {
		if *value == "" {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required config values: %v", missing)
	}

	if servicesFile := os.Getenv("SERVICES_FILE"); servicesFile != "" {
		file, err := os.Open(servicesFile)
		if err != nil {
			return fmt.Errorf("open services file: %w", err)
		}
		defer file.Close()

		var servicesConfig struct {
			Services map[string]string `yaml:"services"`
		}
		if err := yaml.NewDecoder(file).Decode(&servicesConfig); err != nil {
			return fmt.Errorf("decode services file: %w", err)
		}

		if servicesConfig.Services != nil {
			config.Services = servicesConfig.Services
		}
	}

	return nil
}
