package main

import (
	"bufio"
	"fmt"
	"io"
	"log/syslog"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
)

const configPath = "/etc/logpipe.conf"

var facilities = make(map[string]syslog.Priority)
var severities = make(map[string]syslog.Priority)

func printConfig() {
	conf := `[[pipe]]
path = "/tmp/access_log"
facility = "local6"
severity = "info"
tag = "nginx"

[[pipe]]
path = "/tmp/error_log"
facility = "local6"
severity = "err"
tag = "nginx"`

	fmt.Printf("Write configuration file like this:\n---\n%s\n---\nsave in %s\n", conf, configPath)
	os.Exit(1)
}

func init() {
	// Facilities
	facilities["kern"] = syslog.LOG_KERN
	facilities["user"] = syslog.LOG_USER
	facilities["mail"] = syslog.LOG_MAIL
	facilities["daemon"] = syslog.LOG_DAEMON
	facilities["auth"] = syslog.LOG_AUTH
	facilities["syslog"] = syslog.LOG_SYSLOG
	facilities["lpr"] = syslog.LOG_LPR
	facilities["news"] = syslog.LOG_NEWS
	facilities["uucp"] = syslog.LOG_UUCP
	facilities["authpriv"] = syslog.LOG_AUTHPRIV
	facilities["ftp"] = syslog.LOG_FTP
	facilities["cron"] = syslog.LOG_CRON
	facilities["local0"] = syslog.LOG_LOCAL0
	facilities["local1"] = syslog.LOG_LOCAL1
	facilities["local2"] = syslog.LOG_LOCAL2
	facilities["local3"] = syslog.LOG_LOCAL3
	facilities["local4"] = syslog.LOG_LOCAL4
	facilities["local5"] = syslog.LOG_LOCAL5
	facilities["local6"] = syslog.LOG_LOCAL6
	facilities["local7"] = syslog.LOG_LOCAL7

	// Severities
	severities["emerg"] = syslog.LOG_EMERG
	severities["alert"] = syslog.LOG_ALERT
	severities["crit"] = syslog.LOG_CRIT
	severities["err"] = syslog.LOG_ERR
	severities["warning"] = syslog.LOG_WARNING
	severities["notice"] = syslog.LOG_NOTICE
	severities["info"] = syslog.LOG_INFO
	severities["debug"] = syslog.LOG_DEBUG
}

type pipe struct {
	Path     string `toml:"path"`
	Facility string `toml:"facility"`
	Severity string `toml:"severity"`
	Tag      string `toml:"tag"`
}

type config struct {
	Pipe []pipe `toml:"pipe"`
}

func listenPipe(pipe pipe, wg sync.WaitGroup) {
	// Calculate priority

	if pipe.Facility == "" {
		fmt.Printf("Configuration error: %s has no facility set\n", pipe.Path)
		printConfig()
	}
	facility, found := facilities[pipe.Facility]
	if !found {
		fmt.Printf("Configuration error: %s has unknown facility (%s)\n", pipe.Path, pipe.Facility)
		printConfig()
	}

	if pipe.Severity == "" {
		fmt.Printf("Configuration error: %s has no severity set\n", pipe.Path)
		printConfig()
	}
	severity, found := severities[pipe.Severity]
	if !found {
		fmt.Printf("Configuration error: %s has unknown severity (%s)\n", pipe.Path, pipe.Severity)
		printConfig()
	}

	priority := facility | severity

	// Check if pipe already exists
	pipeExists := false
	fileInfo, err := os.Stat(pipe.Path)

	if err == nil {
		if (fileInfo.Mode() & os.ModeNamedPipe) > 0 {
			pipeExists = true
		} else {
			fmt.Printf("%d != %d\n", os.ModeNamedPipe, fileInfo.Mode())
			panic(pipe.Path + " exists, but it's not a named pipe (FIFO)")
		}
	}

	// Try to create pipe if needed
	if !pipeExists {
		err := syscall.Mkfifo(pipe.Path, 0666)
		if err != nil {
			panic(err.Error())
		}
	}

	// Open pipe for reading
	fd, err := os.Open(pipe.Path)
	if err != nil {
		panic(err.Error())
	}
	defer fd.Close()
	reader := bufio.NewReader(fd)

	// Open connection to local syslog
	log, err := syslog.New(priority, pipe.Tag)

	// Loop forever
	for {
		message, err := reader.ReadString(0xa)
		if err != nil && err != io.EOF {
			panic("Reading from pipe failed: " + err.Error())
		}

		if message != "" {
			_, err = log.Write([]byte(message))
			if err != nil {
				panic("Writing to syslog failed: " + err.Error())
			}
		}
	}

	wg.Done()
}

func main() {
	var config config

	// Read the configuration file
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		printConfig()
	}

	// We use a waitgroup to avoid the application exiting
	var wg sync.WaitGroup

	// Start a worker for each pipe
	for _, pipe := range config.Pipe {
		wg.Add(1)
		go listenPipe(pipe, wg)
	}

	// This is a disgusting hack to keep logpipe running without doing anything
	// It can be usefull for automated systems that expect a process to always be running
	if len(config.Pipe) == 0 {
		go func() {
			for {
				time.Sleep(time.Hour)
			}
		}()

		select {}
	}

	// Wait for all workers
	wg.Wait()
}
