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
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/logger"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"
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
	k      *koanf.Koanf
}

func New(configPath string, version string) *AppConfig {
	c := &AppConfig{
		Config: &domain.Config{
			Version:    version,
			ConfigPath: configPath,
		},
		m: new(sync.Mutex),
		k: koanf.New("."),
	}

	c.defaults()
	c.load()
	c.loadFromEnv()

	if c.Config.DownloadLocation == "" {
		log.Fatalf("downloadLocation can't be empty, please provide a valid path to the directory you want your downloads to go to")
	}

	return c
}

func (c *AppConfig) defaults() {
	c.Config.DownloadLocation = ""
	c.Config.NamingTemplate = "{manga:<.>} Ch. {num:3}"
	c.Config.CheckInterval = 15
	c.Config.MonitoredManga = make(map[string]*domain.MonitoredManga)
	c.Config.LogPath = ""
	c.Config.LogLevel = "DEBUG"
	c.Config.LogMaxSize = 50
	c.Config.LogMaxBackups = 3

	// load default values into koanf
	if err := c.k.Load(structs.Provider(c.Config, "yaml"), nil); err != nil {
		log.Fatalf("could not load default values into config: %q", err)
	}
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
						c.Config.CheckInterval = time.Duration(i)
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

	if err := c.k.Load(structs.Provider(c.Config, "yaml"), nil); err != nil {
		log.Fatalf("could not load env vars into config: %q", err)
	}
}

func (c *AppConfig) load() {
	configPath := path.Clean(c.Config.ConfigPath)

	var configFile string

	if configPath != "" {
		if err := c.writeConfig(configPath, "config.yaml"); err != nil {
			log.Printf("config write error: %q", err)
		}

		configFile = path.Join(configPath, "config.yaml")
	} else {
		locations := []string{
			"./config.yaml",
			"$HOME/.config/seasonpackarr/config.yaml",
			"$HOME/.seasonpackarr/config.yaml",
		}

		for _, loc := range locations {
			expandedLoc := os.ExpandEnv(loc)
			if _, err := os.Stat(expandedLoc); err == nil {
				configFile = expandedLoc
				break
			}
		}

		if configFile == "" {
			log.Fatalf("could not find config file")
		}
	}

	if err := c.k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		log.Fatalf("config read error: %q", err)
	}

	if err := c.k.Unmarshal("", c.Config); err != nil {
		log.Fatalf("could not unmarshal config file: %v: err %q", configFile, err)
	}
}

func (c *AppConfig) DynamicReload(log logger.Logger) {
	configFile := path.Join(c.Config.ConfigPath, "config.yaml")

	f := file.Provider(configFile)

	f.Watch(func(event any, err error) {
		if err != nil {
			log.Error().Err(err).Msg("error watching config file")
			return
		}

		c.m.Lock()
		defer c.m.Unlock()

		// create a new koanf instance for reloading
		k := koanf.New(".")

		// load the config file
		if err := k.Load(f, yaml.Parser()); err != nil {
			log.Error().Err(err).Msg("failed to reload config file")
			return
		}

		// unmarshal the updated config into the Config struct
		if err := k.Unmarshal("", c.Config); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal updated config")
			return
		}

		log.SetLogLevel(c.Config.LogLevel)

		log.Debug().Msg("config file reloaded!")
	})
}

func (c *AppConfig) UpdateConfig() error {
	configFile := path.Join(c.Config.ConfigPath, "config.yaml")

	f, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("could not read config configFile: %s: %w", configFile, err)
	}

	lines := strings.Split(string(f), "\n")
	lines = c.processLines(lines)

	output := strings.Join(lines, "\n")
	if err := os.WriteFile(configFile, []byte(output), 0o644); err != nil {
		return fmt.Errorf("could not write config file: %s: %w", configFile, err)
	}

	return nil
}

func (c *AppConfig) processLines(lines []string) []string {
	// keep track of not found values to append at the bottom
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
