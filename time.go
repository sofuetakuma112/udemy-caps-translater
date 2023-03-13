package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseDuration parses an ISO 8601 string representing a duration,
// and returns the resultant golang time.Duration instance.
func ParseDuration(isoDuration string) (float64, error) {
	re := regexp.MustCompile(`^P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)D)?T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:.\d+)?)S)?$`)
	matches := re.FindStringSubmatch(isoDuration)
	if matches == nil {
		return 0, errors.New("input string is of incorrect format")
	}

	seconds := 0.0

	//skipping years and months

	//days
	if matches[3] != "" {
		f, err := strconv.ParseFloat(matches[3], 32)
		if err != nil {
			return 0, err
		}

		seconds += (f * 24 * 60 * 60)
	}
	//hours
	if matches[4] != "" {
		f, err := strconv.ParseFloat(matches[4], 32)
		if err != nil {
			return 0, err
		}

		seconds += (f * 60 * 60)
	}
	//minutes
	if matches[5] != "" {
		f, err := strconv.ParseFloat(matches[5], 32)
		if err != nil {
			return 0, err
		}

		seconds += (f * 60)
	}
	//seconds & milliseconds
	if matches[6] != "" {
		f, err := strconv.ParseFloat(matches[6], 32)
		if err != nil {
			return 0, err
		}

		seconds += f
	}

	return seconds, nil
}

// FormatDuration returns an ISO 8601 duration string.
func FormatDuration(dur time.Duration) string {
	return "PT" + strings.ToUpper(dur.Truncate(time.Millisecond).String())
}

func ms2likeISOFormat(ms int) string {
	nano := ms * 1000000

	t := time.Date(1970, time.January, 1, 0, 0, 0, nano, time.UTC)
	format := "2006-01-02T15:04:05.999Z"
	iso := t.UTC().Format(format)

	if len([]rune(iso)) != len([]rune(format)) {
		idx := strings.Index(iso, ".")
		if idx == -1 {
			// 2006-01-02T15:04:05Z
			iso = iso[:len([]rune(iso))-1] + ".000Z"
		} else {
			// 2006-01-02T15:04:05.0Z
			// 2006-01-02T15:04:05.00Z
			// 2006-01-02T15:04:05.000Z
			mili_str := iso[idx+1 : len([]rune(iso))-1]
			for {
				if len(mili_str) == 3 {
					break
				}
				mili_str = mili_str + "0"
			}
			iso = iso[:idx] + "." + mili_str + "Z"
		}
	}

	trimmedIso := iso[8 : len([]rune(iso))-1]
	day_str := trimmedIso[0:2]
	day, _ := strconv.Atoi(day_str)
	dayStartFromZero := fmt.Sprintf("%02d", day-1)
	isoOnlyTime := trimmedIso[3:]
	return dayStartFromZero + ":" + isoOnlyTime
}

func likeIso2Float(timeString string) float64 {
	// ":"と"."を区切り文字として、時間、分、秒、ミリ秒に分割
	timeParts := strings.Split(timeString, ":")
	secAndMilli := strings.Split(timeParts[2], ".")
	timeInSecs := 0.0

	// 時間、分、秒を秒に変換して加算
	timeInSecs += (toFloat64(timeParts[0]) * 60 * 60) + (toFloat64(timeParts[1]) * 60) + toFloat64(secAndMilli[0])

	// ミリ秒を秒に変換して加算
	timeInSecs += toFloat64(secAndMilli[1]) / 1000

	return timeInSecs
}

// 文字列を数値に変換する関数
func toFloat64(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}
	return f
}
