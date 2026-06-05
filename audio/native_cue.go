package audio

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Track struct {
	TrackNum int
	Title    string
	Performer string
	Rem      map[string]string
	Index01  string // MM:SS:FF
	Index01Seconds float64
}

type CueSheet struct {
	Performer string
	Title     string
	File      string
	Rem       map[string]string
	Tracks    []Track
}

func parseTime(timeStr string) float64 {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 3 {
		return 0
	}
	m, _ := strconv.Atoi(parts[0])
	s, _ := strconv.Atoi(parts[1])
	f, _ := strconv.Atoi(parts[2])
	// 75 frames per second in CD audio
	return float64(m*60) + float64(s) + float64(f)/75.0
}

func ParseCue(cuePath string) (*CueSheet, error) {
	file, err := os.Open(cuePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sheet := &CueSheet{Rem: make(map[string]string)}
	scanner := bufio.NewScanner(file)
	
	var currentTrack *Track

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "REM ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				key := strings.ToUpper(parts[1])
				val := strings.Trim(strings.Join(parts[2:], " "), `"`)
				if currentTrack != nil {
					if currentTrack.Rem == nil {
						currentTrack.Rem = make(map[string]string)
					}
					currentTrack.Rem[key] = val
				} else {
					sheet.Rem[key] = val
				}
			}
		} else if strings.HasPrefix(line, "PERFORMER ") {
			val := strings.Trim(strings.TrimPrefix(line, "PERFORMER "), `"`)
			if currentTrack != nil {
				currentTrack.Performer = val
			} else {
				sheet.Performer = val
			}
		} else if strings.HasPrefix(line, "TITLE ") {
			val := strings.Trim(strings.TrimPrefix(line, "TITLE "), `"`)
			if currentTrack != nil {
				currentTrack.Title = val
			} else {
				sheet.Title = val
			}
		} else if strings.HasPrefix(line, "FILE ") {
			parts := strings.SplitN(line, `"`, 3)
			if len(parts) >= 2 {
				sheet.File = parts[1]
			}
		} else if strings.HasPrefix(line, "TRACK ") {
			if currentTrack != nil {
				sheet.Tracks = append(sheet.Tracks, *currentTrack)
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				num, _ := strconv.Atoi(parts[1])
				currentTrack = &Track{TrackNum: num}
			}
		} else if strings.HasPrefix(line, "INDEX 01 ") {
			if currentTrack != nil {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					currentTrack.Index01 = parts[2]
					currentTrack.Index01Seconds = parseTime(parts[2])
				}
			}
		}
	}

	if currentTrack != nil {
		sheet.Tracks = append(sheet.Tracks, *currentTrack)
	}

	return sheet, scanner.Err()
}