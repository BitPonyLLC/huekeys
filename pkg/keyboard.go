package keyboard

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// RGBColor represents Red Green and Blue values of a color
type RGBColor struct {
	Red   int
	Green int
	Blue  int
}

const RandomColor = "random"
const rgbHexFormat = "%02X%02X%02X"

//go:embed colornames.csv.gz
var colorNamesCSVGZ string

var presetColors = map[string]RGBColor{
	"red":    {255, 0, 0},
	"orange": {255, 128, 0},
	"yellow": {255, 255, 0},
	"green":  {0, 255, 0},
	"aqua":   {25, 255, 223},
	"blue":   {0, 0, 255},
	"pink":   {255, 105, 180},
	"purple": {128, 0, 128},
	"white":  {255, 255, 255},
}

var colorFiles = []string{"color", "color_center", "color_left", "color_right", "color_extra"}
var ledClass = []string{"system76_acpi", "system76"}
var sysFSPath = "/sys/class/leds/%v::kbd_backlight"

type SysPath struct {
	Path  string
	Files [8]string
}

var foundSysPath *SysPath

func LoadEmbeddedColors() error {
	rand.Seed(time.Now().UnixMilli())

	gzReader := strings.NewReader(colorNamesCSVGZ)
	reader, err := gzip.NewReader(gzReader)
	if err != nil {
		return fmt.Errorf("can't read embedded colors: %w", err)
	}
	defer reader.Close()

	firstLine := true
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if firstLine {
			firstLine = false // ignore csv header
			continue
		}
		line := scanner.Text()
		columns := strings.Split(line, ",")
		if len(columns) < 2 {
			log.Warn().Str("line", line).Int("len", len(columns)).Msg("ignoring line from embedded colors")
			continue
		}
		name := strings.ToLower(columns[0])
		hex := columns[1]
		if hex[0] == '#' {
			hex = hex[1:]
		}
		rgb := RGBColor{}
		n, _ := fmt.Sscanf(hex, rgbHexFormat, &rgb.Red, &rgb.Green, &rgb.Blue)
		if n != 3 {
			log.Warn().Str("line", line).Int("n", n).Msg("ignoring line from embedded colors")
			continue
		}
		presetColors[name] = rgb
	}

	return nil
}

// GetColorInHex returns a color in HEX format
func (c RGBColor) GetColorInHex() string {
	return fmt.Sprintf(rgbHexFormat, c.Red, c.Green, c.Blue)
}

func getSysPath() SysPath {
	if foundSysPath != nil {
		return *foundSysPath
	}
	ret := SysPath{"", [8]string{}}
	for _, sub := range ledClass {
		d := fmt.Sprintf(sysFSPath, sub)
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			ret.Path = d
			break
		}
	}
	i := 0
	for _, file := range colorFiles {
		d := fmt.Sprintf("%v/%v", ret.Path, file)
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			ret.Files[i] = file
			i += 1
		}
	}
	foundSysPath = &ret
	return ret
}

func getColorOf(color string) string {
	var red int
	var green int
	var blue int
	n, err := fmt.Sscanf(color, rgbHexFormat, &red, &green, &blue)
	if err != nil || n != 3 {
		return color
	}
	for name, rgb := range presetColors {
		if rgb.Red == red && rgb.Green == green && rgb.Blue == blue {
			return name
		}
	}
	return color
}

func getRandomColor() string {
	return fmt.Sprintf(rgbHexFormat, rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

func EachPresetColor(cb func(name, value string)) {
	keys := []string{}
	for k := range presetColors {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, n := range keys {
		rgb := presetColors[n]
		cb(n, rgb.GetColorInHex())
	}
}

// ColorFileHandler writes a string to colorFiles
func ColorFileHandler(color string) error {
	sys := getSysPath()
	if sys.Path == "" {
		return errors.New("can't get a valid sysfs leds path")
	}
	if presetColor, exists := presetColors[color]; exists {
		color = presetColor.GetColorInHex()
	} else if color == RandomColor {
		color = getRandomColor()
	}
	for _, file := range sys.Files {
		if file == "" {
			continue
		}
		p := fmt.Sprintf("%v/%v", sys.Path, file)
		fh, err := os.OpenFile(p, os.O_RDWR, 0755)
		if err != nil {
			log.Warn().Err(err).Str("path", sys.Path).Msg("can't open")
			continue
		}
		_, err = fh.WriteString(color)
		if err != nil {
			log.Warn().Err(err).Str("path", sys.Path).Msg("can't set color")
			continue
		}
		fh.Close()
	}
	return nil
}

// BrightnessFileHandler writes a hex value to brightness, and returns the bytes written
func BrightnessFileHandler(c string) error {
	sys := getSysPath()
	p := fmt.Sprintf("%v/brightness", sys.Path)

	f, err := os.OpenFile(p, os.O_RDWR, 0755)
	if err != nil {
		return fmt.Errorf("can't open brightness file (%s): %w", p, err)
	}
	defer f.Close()

	_, err = f.WriteString(c)
	if err != nil {
		return fmt.Errorf("can't set brightness value (%s): %w", p, err)
	}

	return nil
}

func GetCurrentColors() (map[string]string, error) {
	sys := getSysPath()
	if sys.Path == "" {
		return nil, errors.New("can't get a valid sysfs leds path")
	}
	ret := map[string]string{}
	for _, file := range sys.Files {
		if file == "" {
			continue
		}
		p := fmt.Sprintf("%v/%v", sys.Path, file)
		fh, err := os.Open(p)
		if err != nil {
			log.Print(err)
			continue
		}
		defer fh.Close()
		buf := make([]byte, 6)
		_, err = fh.Read(buf)
		if err != nil {
			log.Warn().Err(err).Str("path", p).Msg("read failed")
			continue
		}
		ret[file] = getColorOf(string(buf))
	}
	return ret, nil
}

func GetCurrentBrightness() (string, error) {
	sys := getSysPath()
	p := fmt.Sprintf("%v/brightness", sys.Path)
	f, err := os.Open(p)
	if err != nil {
		return "", fmt.Errorf("can't open %s: %w", p, err)
	}
	defer f.Close()
	buf := make([]byte, 3)
	_, err = f.Read(buf)
	if err != nil {
		return "", fmt.Errorf("can't read %s: %w", p, err)
	}
	// make sure we don't include the null byte if it's included
	length := bytes.IndexByte(buf, 0)
	if length < 0 {
		length = len(buf)
	}
	return strings.TrimSpace(string(buf[0:length])), nil
}
