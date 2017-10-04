package main

import (
	log "github.com/cihub/seelog"
	"gopkg.in/ini.v1"
)

func main() {
	initLogger()
	defer log.Flush()
	port := initConfig()

}

func initLogger() {
	logger, err := log.LoggerFromConfigAsFile("config/seelog.xml")
	if err != nil {
		panic(err)
	}
	log.ReplaceLogger(logger)
}

func initConfig() string {
	iniCongig, err := ini.Load("config/api.ini")
	if err != nil {
		panic(err)
	}
	if sec, err := iniCongig.GetSection("server"); err == nil {
		if sec.Haskey("port") {
			port, _ := sec.GetKey("port")
			return port.String()
		}
	} else {
		panic(err)
	}

	return ""
}
