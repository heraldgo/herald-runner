package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/heraldgo/heraldd/util"
)

var log *logrus.Logger

var cfg struct {
	LogLevel      string `yaml:"log_level"`
	LogTimeFormat string `yaml:"time_format"`
	LogOutput     string `yaml:"log_output"`
	WorkDir       string `yaml:"work_dir"`
	Secret        string `yaml:"secret"`
	UnixSocket    string `yaml:"unix_socket"`
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
}

func loadConfigFile(configFile string) error {
	buffer, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(buffer, &cfg)
	if err != nil {
		return err
	}

	return nil
}

func setupLog(logFile **os.File) {
	level := logrus.InfoLevel
	timeFormat := "2006-01-02 15:04:05.000 -0700 MST"

	levelLogrusTemp, err := logrus.ParseLevel(cfg.LogLevel)
	if err == nil {
		level = levelLogrusTemp
	}

	if cfg.LogTimeFormat != "" {
		timeFormat = cfg.LogTimeFormat
	}

	log.SetLevel(level)
	log.SetFormatter(&util.SimpleFormatter{
		TimeFormat: timeFormat,
	})

	if cfg.LogOutput != "" {
		logDir := filepath.Dir(cfg.LogOutput)
		if logDir != "" {
			os.MkdirAll(logDir, 0755)
			if err != nil {
				log.Errorf(`Create log directory "%s" failed: %s`, logDir, err)
				return
			}
		}

		f, err := os.OpenFile(cfg.LogOutput, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Errorf(`[HeraldRunner] Create log file "%s" error: %s`, cfg.LogOutput, err)
		} else {
			log.SetOutput(f)
			*logFile = f
		}
	}
}

func newRunner() *runner {
	s := &runner{}

	s.ServerHeader = "herald-runner"
	s.UnixSocket = cfg.UnixSocket
	s.Host = cfg.Host
	s.Port = cfg.Port
	s.secret = cfg.Secret
	s.exeGit.WorkDir = cfg.WorkDir

	if s.Port == 0 && s.UnixSocket == "" {
		s.Host = "127.0.0.1"
		s.Port = 8124
	}

	s.SetLogger(&util.PrefixLogger{
		Logger: log,
		Prefix: "[HeraldRunner]",
	})

	return s
}

func printVersion() {
	fmt.Printf("Herald Runner %s\n", Version)
}

func main() {
	flagVersion := flag.Bool("version", false, "Print Herald Runner version")
	flagConfigFile := flag.String("config", "config.yml", "Configuration file path")
	flag.Parse()

	if *flagVersion {
		printVersion()
		return
	}

	log = logrus.New()

	err := loadConfigFile(*flagConfigFile)
	if err != nil {
		log.Errorf(`[HeraldRunner] Load config file "%s" error: %s`, *flagConfigFile, err)
		return
	}

	var logFile *os.File
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	setupLog(&logFile)

	log.Infoln(strings.Repeat("=", 80))
	log.Infoln("[HeraldRunner] Initialize...")

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	s := newRunner()

	log.Infoln("[HeraldRunner] Start...")

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Run(ctx)
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Infoln("[HeraldRunner] Shutdown...")

	cancel()

	wg.Wait()

	log.Infoln("[HeraldRunner] Exiting...")
	log.Infoln(strings.Repeat("-", 80))
}
