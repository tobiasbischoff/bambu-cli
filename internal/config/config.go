package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Profile struct {
	IP             string `json:"ip,omitempty"`
	Serial         string `json:"serial,omitempty"`
	AccessCodeFile string `json:"access_code_file,omitempty"`
	Username       string `json:"username,omitempty"`
	NoCamera       bool   `json:"no_camera,omitempty"`
	MQTTPort       int    `json:"mqtt_port,omitempty"`
	FTPPort        int    `json:"ftp_port,omitempty"`
	CameraPort     int    `json:"camera_port,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type Config struct {
	DefaultProfile string             `json:"default_profile,omitempty"`
	Profiles       map[string]Profile `json:"profiles,omitempty"`
}

func Empty() Config {
	return Config{Profiles: map[string]Profile{}}
}

func Read(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Empty(), nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return cfg, nil
}

func Merge(base Config, override Config) Config {
	out := base
	if out.Profiles == nil {
		out.Profiles = map[string]Profile{}
	}
	if override.DefaultProfile != "" {
		out.DefaultProfile = override.DefaultProfile
	}
	for name, p := range override.Profiles {
		if existing, ok := out.Profiles[name]; ok {
			out.Profiles[name] = mergeProfile(existing, p)
		} else {
			out.Profiles[name] = p
		}
	}
	return out
}

func mergeProfile(base Profile, override Profile) Profile {
	out := base
	if override.IP != "" {
		out.IP = override.IP
	}
	if override.Serial != "" {
		out.Serial = override.Serial
	}
	if override.AccessCodeFile != "" {
		out.AccessCodeFile = override.AccessCodeFile
	}
	if override.Username != "" {
		out.Username = override.Username
	}
	if override.MQTTPort != 0 {
		out.MQTTPort = override.MQTTPort
	}
	if override.FTPPort != 0 {
		out.FTPPort = override.FTPPort
	}
	if override.CameraPort != 0 {
		out.CameraPort = override.CameraPort
	}
	if override.TimeoutSeconds != 0 {
		out.TimeoutSeconds = override.TimeoutSeconds
	}
	if override.NoCamera {
		out.NoCamera = true
	}
	return out
}

func Save(path string, cfg Config) error {
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
