package awml

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var isoDuration = regexp.MustCompile(`^P(?:(\d+(?:[.,]\d+)?)W|(?:(\d+(?:[.,]\d+)?)D)?(?:T(?:(\d+(?:[.,]\d+)?)H)?(?:(\d+(?:[.,]\d+)?)M)?(?:(\d+(?:[.,]\d+)?)S)?)?)$`)

func validDuration(text string) bool {
	_, ok := duration(text)
	return ok
}

// duration reads the fixed-length parts of an ISO 8601 duration. A fraction may appear only on
// its smallest unit, as ISO 8601 requires.
func duration(text string) (time.Duration, bool) {
	match := isoDuration.FindStringSubmatch(text)
	if match == nil {
		return 0, false
	}
	last := 0
	for i := 1; i < len(match); i++ {
		if match[i] != "" {
			last = i
		}
	}
	if last == 0 || strings.HasSuffix(text, "T") {
		return 0, false
	}
	var total float64
	units := []float64{0, 7 * 24 * 3600, 24 * 3600, 3600, 60, 1}
	for i := 1; i < len(match); i++ {
		if match[i] == "" {
			continue
		}
		if i != last && strings.ContainsAny(match[i], ".,") {
			return 0, false
		}
		n, err := strconv.ParseFloat(strings.Replace(match[i], ",", ".", 1), 64)
		if err != nil {
			return 0, false
		}
		total += n * units[i]
	}
	return time.Duration(total * float64(time.Second)), true
}
