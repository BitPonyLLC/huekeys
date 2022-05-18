package keyboard

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"
	"unicode"

	"github.com/bambash/sys76-kb/internal/image_matcher"
)

// BrightnessPulse continuously dials up and down brightness
func BrightnessPulse(delay time.Duration) {
	if delay == 0 {
		delay = 25 * time.Millisecond
	}

	for {
		for i := 255; i >= 0; i-- {
			s := strconv.Itoa(i)
			BrightnessFileHandler(s)
			time.Sleep(delay)
		}
		for i := 1; i <= 255; i++ {
			s := strconv.Itoa(i)
			BrightnessFileHandler(s)
			time.Sleep(delay)
		}
	}
}

// InfiniteRainbow generates... an infinite rainbow
func InfiniteRainbow(delay time.Duration) {
	if delay == 0 {
		delay = time.Nanosecond
	}

	var currentColor string
	var currentColorOffset int

	currentColors := GetCurrentColors()
	for _, v := range currentColors {
		// assume all groups are set to the same color for now and simply grab the first one
		currentColor = v
		break
	}

	colors := make([]string, 0, 6)

	add := func(r, g, b int) {
		c := RGBColor{r, g, b}
		ch := c.GetColorInHex()
		if ch == currentColor {
			currentColorOffset = len(colors)
		}
		colors = append(colors, ch)
	}

	// generate range of rainbow values
	for i := 0; i <= 255; i++ {
		add(255, i, 0)
	}

	for i := 255; i >= 0; i-- {
		add(i, 255, 0)
	}

	for i := 0; i <= 255; i++ {
		add(0, 255, i)
	}

	for i := 255; i >= 0; i-- {
		add(0, i, 255)
	}

	for i := 0; i <= 255; i++ {
		add(i, 0, 255)
	}

	for i := 255; i >= 0; i-- {
		add(255, 0, i)
	}

	for {
		for i := currentColorOffset; i < len(colors); i++ {
			c := colors[i]
			ColorFileHandler(c)
			time.Sleep(delay)
		}
		currentColorOffset = 0 // only used on first pass
	}
}

// InfinitRandom sets the keyboard colors to random values forever
func InfiniteRandom(delay time.Duration) {
	if delay == 0 {
		delay = time.Second
	}

	for {
		ColorFileHandler(RandomColor)
		time.Sleep(delay)
	}
}

var pictureURIMonitorRE = regexp.MustCompile(`^\s*picture-uri(?:-dark)?:\s*'([^']+)'\s*$`)

func MatchDesktopBackground() {
	// TODO: consider using D-Bus directly instead of gsettings...

	colorScheme, err := getDesktopSetting("interface", "color-scheme")
	if err != nil {
		return
	}

	pictureGroup := "picture-uri"
	if colorScheme == "prefer-dark" {
		pictureGroup += "-dark"
	}

	pictureURIStr, err := getDesktopSetting("background", pictureGroup)
	if err != nil {
		return
	}

	pictureURL, err := url.Parse(pictureURIStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't parse %s: %v\n", pictureURIStr, err)
		return
	}

	setColorFrom(pictureURL.Path)

	cmd := newDesktopSettingCmd("monitor", "background", pictureGroup)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't get stdout of monitor for gnome background picture url: %v\n", err)
		return
	}

	err = cmd.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't start monitor for gnome background picture url: %v\n", err)
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		m := pictureURIMonitorRE.FindStringSubmatch(line)
		if m == nil || len(m) < 2 || m[1] == "" {
			fmt.Fprintf(os.Stderr, "ignoring line found in monitor output: %s\n", line)
			continue
		}
		setColorFrom(m[1])
	}
}

func newDesktopSettingCmd(action, group, key string) *exec.Cmd {
	cmdName := "gsettings"
	args := []string{}

	// if running as root via sudo, need to ask for the user's desktop image...
	if os.Getuid() == 0 {
		sudoUser := os.Getenv("SUDO_USER")
		if sudoUser != "" {
			cmdName = "sudo"
			args = []string{"-Eu", sudoUser, "gsettings"}
		}
	}

	group = "org.gnome.desktop." + group
	args = append(args, action, group, key)
	cmd := exec.Command(cmdName, args...)
	fmt.Printf("%s\n", cmd)
	return cmd
}

func getDesktopSetting(group, key string) (string, error) {
	val, err := newDesktopSettingCmd("get", group, key).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't get %s %s: %v\n", group, key, err)
		return "", err
	}

	val = bytes.TrimFunc(val, func(r rune) bool { return unicode.IsSpace(r) || r == '\'' })
	return string(val), nil
}

func setColorFrom(u string) {
	pictureURL, err := url.Parse(u)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't parse picture uri (%s): %v\n", u, err)
		return
	}

	color, err := image_matcher.GetDominantColorOf(pictureURL.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't get dominant color: %v\n", err)
		return
	}

	ColorFileHandler(color)
}
