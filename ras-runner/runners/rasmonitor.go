package runners

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	plug "github.com/Dewberry/papigoplug/papigoplug"
)

func getPercentComplete(s string, cv *map[string]float64) (bool, string) {
	logEntry := strings.Split(s, "=")
	pctComplete := strings.TrimSpace(logEntry[1])
	if num, err := strconv.ParseFloat(pctComplete, 32); err == nil {
		for k, val := range *cv {
			if math.Abs(val-num) < 0.01 {
				delete(*cv, k)
				return true, k
			}
		}
	}
	return false, ""
}

func rasPctLog(startLogging *int, message string, checkValues *map[string]float64) {

	// RAS hack to only print progress every 10%
	if strings.Contains(message, "LABEL= Unsteady Flow Computations") {
		// trigger logging
		*startLogging += 1
	}

	if strings.Contains(message, "LABEL= Unsteady Flow Warmup") {
		// trigger logging
		*startLogging -= 1
	}

	if *startLogging > 0 && strings.Contains(message, "PROGRESS") {
		msg, text := getPercentComplete(message, checkValues)

		if msg {
			pctMsg := fmt.Sprintf("MODEL RUN PROGRESS = %s", text)
			plug.Log.Infof(pctMsg)

		}
	}
}
