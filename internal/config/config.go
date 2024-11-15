package config

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"mangarr/internal/domain"
	"mangarr/internal/logger"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
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
  Solo Max-Level Newbie:
    # Source from where the manga should be downloaded
    #
    source: "asurascans"

    # URL of the manga on Asura Scans
    #
    manga: "https://asuracomic.net/series/solo-max-level-newbie-31f980f5"

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

func (c *AppConfig) writeConfig(configPath string, configFile string) error {
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

type Config interface {
	UpdateConfig() error
	DynamicReload(log logger.Logger)
}

type AppConfig struct {
	Config *domain.Config
	m      *sync.Mutex
}

func New(configPath string, version string) *AppConfig {
	c := &AppConfig{
		m: new(sync.Mutex),
	}
	c.defaults()
	c.Config = &domain.Config{
		Version:    version,
		ConfigPath: configPath,
	}

	c.load(configPath)
	c.loadFromEnv()

	if c.Config.DownloadLocation == "" {
		log.Fatalf("downloadLocation can't be empty, please provide a valid path to the directory you want your downloads to go to")
	}

	return c
}

func (c *AppConfig) defaults() {
	viper.SetDefault("downloadLocation", "")
	viper.SetDefault("namingTemplate", "{manga:<.>} Ch. {num:3}")
	viper.SetDefault("checkInterval", 15)
	viper.SetDefault("monitoredManga", make(map[string]*domain.MonitoredManga))
	viper.SetDefault("logPath", "")
	viper.SetDefault("logLevel", "DEBUG")
	viper.SetDefault("logMaxSize", 50)
	viper.SetDefault("logMaxBackups", 3)
}

func (c *AppConfig) loadFromEnv() {
	prefix := "MANGARR__"

	envs := os.Environ()
	for _, env := range envs {
		if strings.HasPrefix(env, prefix) {
			envPair := strings.SplitN(env, "=", 2)

			if envPair[1] != "" {
				switch envPair[0] {
				case prefix + "DOWNLOAD_LOCATION":
					c.Config.DownloadLocation = envPair[1]
				case prefix + "NAMING_TEMPLATE":
					c.Config.NamingTemplate = envPair[1]
				case prefix + "CHECK_INTERVAL":
					if i, _ := strconv.ParseInt(envPair[1], 10, 32); i > 0 {
						c.Config.CheckInterval = int(i)
					}
				case prefix + "LOG_LEVEL":
					c.Config.LogLevel = envPair[1]
				case prefix + "LOG_PATH":
					c.Config.LogPath = envPair[1]
				case prefix + "LOG_MAX_SIZE":
					if i, _ := strconv.ParseInt(envPair[1], 10, 32); i > 0 {
						c.Config.LogMaxSize = int(i)
					}
				case prefix + "LOG_MAX_BACKUPS":
					if i, _ := strconv.ParseInt(envPair[1], 10, 32); i > 0 {
						c.Config.LogMaxBackups = int(i)
					}
				}
			}
		}
	}
}

func (c *AppConfig) load(configPath string) {
	viper.SetConfigType("yaml")

	// clean trailing slash from configPath
	configPath = path.Clean(configPath)
	if configPath != "" {
		// check if path and file exists
		// if not, create path and file
		if err := c.writeConfig(configPath, "config.yaml"); err != nil {
			log.Printf("write error: %q", err)
		}

		viper.SetConfigFile(path.Join(configPath, "config.yaml"))
	} else {
		viper.SetConfigName("config")

		// Search config in directories
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.config/mangarr")
		viper.AddConfigPath("$HOME/.mangarr")
	}

	// read config
	if err := viper.ReadInConfig(); err != nil {
		log.Printf("config read error: %q", err)
	}

	if err := viper.Unmarshal(c.Config); err != nil {
		log.Fatalf("Could not unmarshal config file: %v: err %q", viper.ConfigFileUsed(), err)
	}
}

func (c *AppConfig) DynamicReload(log logger.Logger) {
	viper.WatchConfig()

	viper.OnConfigChange(func(_ fsnotify.Event) {
		c.m.Lock()
		defer c.m.Unlock()

		logLevel := viper.GetString("logLevel")
		c.Config.LogLevel = logLevel
		log.SetLogLevel(c.Config.LogLevel)

		logPath := viper.GetString("logPath")
		c.Config.LogPath = logPath

		log.Debug().Msg("config file reloaded!")
	})
}

func (c *AppConfig) UpdateConfig() error {
	filePath := path.Join(c.Config.ConfigPath, "config.yaml")

	f, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read config filePath: %s: %w", filePath, err)
	}

	lines := strings.Split(string(f), "\n")
	lines = c.processLines(lines)

	output := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(output), 0o644); err != nil {
		return fmt.Errorf("could not write config file: %s: %w", filePath, err)
	}

	return nil
}

func (c *AppConfig) processLines(lines []string) []string {
	// keep track of not found values to append at bottom
	var (
		foundLineLogLevel = false
		foundLineLogPath  = false
	)

	for i, line := range lines {
		if !foundLineLogLevel && strings.Contains(line, "logLevel:") {
			lines[i] = fmt.Sprintf(`logLevel: "%s"`, c.Config.LogLevel)
			foundLineLogLevel = true
		}
		if !foundLineLogPath && strings.Contains(line, "logPath:") {
			if c.Config.LogPath == "" {
				lines[i] = `#logPath: ""`
			} else {
				lines[i] = fmt.Sprintf(`logPath: "%s"`, c.Config.LogPath)
			}
			foundLineLogPath = true
		}
	}

	if !foundLineLogLevel {
		lines = append(lines, "# Log level")
		lines = append(lines, "#")
		lines = append(lines, `# Default: "DEBUG"`)
		lines = append(lines, "#")
		lines = append(lines, `# Options: "ERROR", "DEBUG", "INFO", "WARN", "TRACE"`)
		lines = append(lines, "#")
		lines = append(lines, fmt.Sprintf(`logLevel: "%s"`, c.Config.LogLevel))
	}

	if !foundLineLogPath {
		lines = append(lines, "# Log Path")
		lines = append(lines, "#")
		lines = append(lines, "# Optional")
		lines = append(lines, "#")
		if c.Config.LogPath == "" {
			lines = append(lines, `#logPath: ""`)
		} else {
			lines = append(lines, fmt.Sprintf(`logPath: "%s"`, c.Config.LogPath))
		}
	}

	return lines
}
