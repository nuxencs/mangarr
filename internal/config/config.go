package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/logger"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

var configTemplate = `# config.yaml

# Download Location
# Needs to be filled out correctly, e.g. "/data/downloads/manga"
#
# Default: ""
#
downloadLocation: ""

# Naming Template
# This can be used to change how the downloaded chapter will be named
# The default will result something like this: Manga Ch. 001 - Chapter Title
#
# Default: {manga:<.>} Ch. {num:3}{title: - <.>}
#
namingTemplate: "{manga:<.>} Ch. {num:3}{title: - <.>}"

# Check interval in minutes
#
# Default: 15
#
checkInterval: 15

# Enable pprof endpoint for runtime profiling
#
# Default: false
#
pprofEnabled: false

# pprof endpoint address
#
# Default: "127.0.0.1:6060"
#
pprofAddress: "127.0.0.1:6060"

# Monitored Manga
# Here you can define which manga you want to monitor
#
monitoredManga:
  # Custom name you can give the entry to easily distinguish between them
  #
  One Piece:
    # Source from where the manga should be downloaded
    #
    source: "tcbscans"

    # Name of the manga on TCB Scans
    #
    manga: "One Piece"

  # Custom name you can give the entry to easily distinguish between them
  #
  Isekai Ojisan:
    # Source from where the manga should be downloaded
    #
    source: "mangadex"

    # ID of the manga on MangaDex
    #
    manga: "d8f1d7da-8bb1-407b-8be3-10ac2894d3c6"

    # ID of the scanlation group on MangaDex
    #
    group: "310361d7-52dd-4848-9b36-2eb4fcc95e83"

    # Language of the manga on MangaDex
    #
    language: "en"

    # Overwrite can be used to overwrite the parsed manga name
    #
    overwrite: "Uncle from Another World"

  # Custom name you can give the entry to easily distinguish between them
  #
  Kagurabachi:
    # Source from where the manga should be downloaded
    #
    source: "mangaplus"

    # ID of the manga on MangaPlus
    #
    manga: "100274"

  # Custom name you can give the entry to easily distinguish between them
  #
  Solo Leveling Ragnarok:
    # Source from where the manga should be downloaded
    #
    source: "flamecomics"

    # URL of the manga on Flame Comics
    #
    manga: "https://flamecomics.xyz/series/solo-leveling-ragnarok/"

  # Custom name you can give the entry to easily distinguish between them
  #
  One Punch Man:
    # Source from where the manga should be downloaded
    #
    source: "cubari"

    # URL of the gist for the manga on Cubari
    #
    manga: "https://git.io/OPM"

    # ID of the scanlation group on MangaDex
    #
    group: "/r/OnePunchMan"

# mangarr logs file
# If not defined, logs to stdout
# Make sure to use forward slashes and include the filename with extension. e.g. "logs/mangarr.log", "C:/mangarr/logs/mangarr.log"
#
# Optional
#
#logPath: ""

# Log level
#
# Default: "DEBUG"
#
# Options: "ERROR", "DEBUG", "INFO", "WARN", "TRACE"
#
logLevel: "DEBUG"

# Log Max Size
#
# Default: 50
#
# Max log size in megabytes
#
#logMaxSize: 50

# Log Max Backups
#
# Default: 3
#
# Max amount of old log files
#
#logMaxBackups = 3
`

func writeConfig(configPath string, configFile string) error {
	cfgPath := filepath.Join(configPath, configFile)

	// check if configPath exists, if not create it
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(configPath, os.ModePerm)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	// check if config exists, if not create it
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {

		f, err := os.Create(cfgPath)
		if err != nil { // perm 0666
			// handle failed create
			log.Printf("error creating file: %q", err)
			return err
		}
		defer f.Close()

		if _, err = f.WriteString(configTemplate); err != nil {
			log.Printf("error writing contents to file: %v %q", configPath, err)
			return err
		}

		return f.Sync()
	}

	return nil
}

type AppConfig struct {
	current    atomic.Pointer[domain.Config]
	configFile string
	version    string
}

func Load(configPath string, version string) (*AppConfig, error) {
	configFile, err := resolveConfigFile(configPath)
	if err != nil {
		return nil, err
	}

	c := &AppConfig{configFile: configFile, version: version}
	snapshot, err := c.loadSnapshot()
	if err != nil {
		return nil, err
	}
	c.current.Store(snapshot)

	return c, nil
}

func defaultConfig(version, configFile string) domain.Config {
	return domain.Config{
		Version:        version,
		ConfigPath:     filepath.Dir(configFile),
		NamingTemplate: "{manga:<.>} Ch. {num:3}{title: - <.>}",
		CheckInterval:  15,
		PprofAddress:   "127.0.0.1:6060",
		MonitoredManga: make(map[string]*domain.MonitoredManga),
		LogLevel:       "DEBUG",
		LogMaxSize:     50,
		LogMaxBackups:  3,
	}
}

func resolveConfigFile(configPath string) (string, error) {
	if configPath != "" {
		cleanPath := filepath.Clean(configPath)
		if err := writeConfig(cleanPath, "config.yaml"); err != nil {
			return "", fmt.Errorf("writing config template: %w", err)
		}

		return filepath.Join(cleanPath, "config.yaml"), nil
	}

	locations := []string{
		"./config.yaml",
		"$HOME/.config/mangarr/config.yaml",
		"$HOME/.mangarr/config.yaml",
	}
	for _, location := range locations {
		expanded := os.ExpandEnv(location)
		if _, err := os.Stat(expanded); err == nil {
			return expanded, nil
		}
	}

	return "", fmt.Errorf("could not find config file")
}

func (c *AppConfig) loadSnapshot() (*domain.Config, error) {
	cfg := defaultConfig(c.version, c.configFile)
	k := koanf.New(".")
	if err := k.Load(structs.Provider(&cfg, "yaml"), nil); err != nil {
		return nil, fmt.Errorf("loading config defaults: %w", err)
	}
	if err := k.Load(file.Provider(c.configFile), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", c.configFile, err)
	}
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("decoding config file %s: %w", c.configFile, err)
	}

	applyEnvironment(&cfg)
	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validating config file %s: %w", c.configFile, err)
	}

	snapshot := cloneConfig(cfg)
	return &snapshot, nil
}

func applyEnvironment(cfg *domain.Config) {
	const prefix = "MANGARR__"

	envs := os.Environ()
	for _, env := range envs {
		if !strings.HasPrefix(env, prefix) {
			continue
		}

		envPair := strings.SplitN(env, "=", 2)
		if len(envPair) != 2 || envPair[1] == "" {
			continue
		}

		switch envPair[0] {
		case prefix + "DOWNLOAD_LOCATION":
			cfg.DownloadLocation = envPair[1]
		case prefix + "NAMING_TEMPLATE":
			cfg.NamingTemplate = envPair[1]
		case prefix + "CHECK_INTERVAL":
			if i, err := strconv.ParseInt(envPair[1], 10, 32); err == nil {
				cfg.CheckInterval = time.Duration(i)
			}
		case prefix + "PPROF_ENABLED":
			if enabled, err := strconv.ParseBool(envPair[1]); err == nil {
				cfg.PprofEnabled = enabled
			}
		case prefix + "PPROF_ADDRESS":
			cfg.PprofAddress = envPair[1]
		case prefix + "LOG_LEVEL":
			cfg.LogLevel = strings.ToUpper(envPair[1])
		case prefix + "LOG_PATH":
			cfg.LogPath = envPair[1]
		case prefix + "LOG_MAX_SIZE":
			if i, err := strconv.ParseInt(envPair[1], 10, 32); err == nil {
				cfg.LogMaxSize = int(i)
			}
		case prefix + "LOG_MAX_BACKUPS":
			if i, err := strconv.ParseInt(envPair[1], 10, 32); err == nil {
				cfg.LogMaxBackups = int(i)
			}
		}
	}
}

func validate(cfg domain.Config) error {
	if cfg.DownloadLocation == "" {
		return fmt.Errorf("downloadLocation cannot be empty")
	}
	if cfg.NamingTemplate == "" {
		return fmt.Errorf("namingTemplate cannot be empty")
	}
	if cfg.CheckInterval <= 0 {
		return fmt.Errorf("checkInterval must be greater than zero")
	}
	if cfg.PprofEnabled && cfg.PprofAddress == "" {
		return fmt.Errorf("pprofAddress cannot be empty when pprof is enabled")
	}
	switch cfg.LogLevel {
	case "ERROR", "DEBUG", "INFO", "WARN", "TRACE":
	default:
		return fmt.Errorf("unsupported logLevel %q", cfg.LogLevel)
	}
	if cfg.LogMaxSize <= 0 {
		return fmt.Errorf("logMaxSize must be greater than zero")
	}
	if cfg.LogMaxBackups <= 0 {
		return fmt.Errorf("logMaxBackups must be greater than zero")
	}
	for name, manga := range cfg.MonitoredManga {
		if manga == nil {
			return fmt.Errorf("monitoredManga %q cannot be null", name)
		}
	}

	return nil
}

func (c *AppConfig) Snapshot() domain.Config {
	return cloneConfig(*c.current.Load())
}

func cloneConfig(cfg domain.Config) domain.Config {
	clone := cfg
	clone.MonitoredManga = make(map[string]*domain.MonitoredManga, len(cfg.MonitoredManga))
	for name, manga := range cfg.MonitoredManga {
		mangaClone := *manga
		clone.MonitoredManga[name] = &mangaClone
	}

	return clone
}

func (c *AppConfig) DynamicReload(log logger.Logger) (<-chan struct{}, error) {
	reloaded := make(chan struct{}, 1)
	f := file.Provider(c.configFile)

	err := f.Watch(func(_ any, watchErr error) {
		if watchErr != nil {
			log.Error().Err(watchErr).Msg("error watching config file")
			return
		}

		snapshot, err := c.loadSnapshot()
		if err != nil {
			log.Error().Err(err).Msg("config reload rejected")
			return
		}

		c.current.Store(snapshot)
		log.SetLogLevel(snapshot.LogLevel)
		select {
		case reloaded <- struct{}{}:
		default:
		}
		log.Debug().Msg("config file reloaded")
	})
	if err != nil {
		return nil, fmt.Errorf("watching config file %s: %w", c.configFile, err)
	}

	return reloaded, nil
}

func (c *AppConfig) UpdateConfig() error {
	f, err := os.ReadFile(c.configFile)
	if err != nil {
		return fmt.Errorf("could not read config file %s: %w", c.configFile, err)
	}

	lines := strings.Split(string(f), "\n")
	lines = processLines(lines, c.Snapshot())

	output := strings.Join(lines, "\n")
	if err := os.WriteFile(c.configFile, []byte(output), 0o644); err != nil {
		return fmt.Errorf("could not write config file %s: %w", c.configFile, err)
	}

	return nil
}

func processLines(lines []string, cfg domain.Config) []string {
	// keep track of not found values to append at the bottom
	var (
		foundLineLogLevel     = false
		foundLineLogPath      = false
		foundLinePprofEnabled = false
		foundLinePprofAddress = false
	)

	for i, line := range lines {
		if !foundLineLogLevel && strings.Contains(line, "logLevel:") {
			lines[i] = fmt.Sprintf(`logLevel: "%s"`, cfg.LogLevel)
			foundLineLogLevel = true
		}
		if !foundLineLogPath && strings.Contains(line, "logPath:") {
			if cfg.LogPath == "" {
				lines[i] = `#logPath: ""`
			} else {
				lines[i] = fmt.Sprintf(`logPath: "%s"`, cfg.LogPath)
			}
			foundLineLogPath = true
		}
		if !foundLinePprofEnabled && strings.Contains(line, "pprofEnabled:") {
			lines[i] = fmt.Sprintf(`pprofEnabled: %t`, cfg.PprofEnabled)
			foundLinePprofEnabled = true
		}
		if !foundLinePprofAddress && strings.Contains(line, "pprofAddress:") {
			lines[i] = fmt.Sprintf(`pprofAddress: "%s"`, cfg.PprofAddress)
			foundLinePprofAddress = true
		}
	}

	if !foundLineLogLevel {
		lines = append(lines, "# Log level")
		lines = append(lines, "#")
		lines = append(lines, `# Default: "DEBUG"`)
		lines = append(lines, "#")
		lines = append(lines, `# Options: "ERROR", "DEBUG", "INFO", "WARN", "TRACE"`)
		lines = append(lines, "#")
		lines = append(lines, fmt.Sprintf(`logLevel: "%s"`, cfg.LogLevel))
	}

	if !foundLineLogPath {
		lines = append(lines, "# Log Path")
		lines = append(lines, "#")
		lines = append(lines, "# Optional")
		lines = append(lines, "#")
		if cfg.LogPath == "" {
			lines = append(lines, `#logPath: ""`)
		} else {
			lines = append(lines, fmt.Sprintf(`logPath: "%s"`, cfg.LogPath))
		}
	}

	if !foundLinePprofEnabled {
		lines = append(lines, "# Enable pprof endpoint for runtime profiling")
		lines = append(lines, "#")
		lines = append(lines, "# Default: false")
		lines = append(lines, "#")
		lines = append(lines, fmt.Sprintf(`pprofEnabled: %t`, cfg.PprofEnabled))
	}

	if !foundLinePprofAddress {
		lines = append(lines, "# pprof endpoint address")
		lines = append(lines, "#")
		lines = append(lines, `# Default: "127.0.0.1:6060"`)
		lines = append(lines, "#")
		lines = append(lines, fmt.Sprintf(`pprofAddress: "%s"`, cfg.PprofAddress))
	}

	return lines
}
