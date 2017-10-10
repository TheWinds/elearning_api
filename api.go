package elearningapi

import (
	log "github.com/cihub/seelog"
)

// IsDeubg weather to show the log
var IsDeubg bool

func init() {
	if IsDeubg {
		initLogger()
		defer log.Flush()
	}
}

func initLogger() {
	config := `<seelog minlevel="debug">
	<outputs>
	<console formatid="colored"/>
</outputs>
<formats>
	<format id="colored"  format="%EscM(46)[%Level]%EscM(49) %Msg%n%EscM(0)"/>
</formats>
</seelog>`
	logger, err := log.LoggerFromConfigAsString(config)
	if err != nil {
		panic(err)
	}
	log.ReplaceLogger(logger)
}
