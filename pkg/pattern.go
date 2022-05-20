package keyboard

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
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

	f, _ := os.Create("barf.html")
	f.WriteString("<html><body>\n")

	add := func(r, g, b int) {
		c := RGBColor{r, g, b}
		ch := c.GetColorInHex()
		f.WriteString(fmt.Sprintf(`<h5 style="background-color:#%s">%s</h5>`+"\n", ch, ch))
		if ch == currentColor {
			currentColorOffset = len(colors)
		}
		colors = append(colors, ch)
	}

	// generate range of rainbow values ("cold" to "hot")
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

	f.WriteString("</body></html>\n")
	f.Close()

	os.Exit(42)
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

// MonitorCPU sets the keyboard colors according to CPU utilization
func MonitorCPU(delay time.Duration) {
	if delay == 0 {
		delay = time.Second
	}

	for {
		previous := getCPUStats()
		time.Sleep(delay)
		current := getCPUStats()
		cpuPercentage := float64(current.active-previous.active) / float64(current.total-previous.total)
		i := int(math.Round(float64(len(coldHotColors)-1) * cpuPercentage))
		color := coldHotColors[i]
		ColorFileHandler(color)
	}
}

type cpuStats struct {
	active int
	total  int
}

func getCPUStats() *cpuStats {
	f, err := os.Open("/proc/stat")
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't open system stats: %v\n", err)
		return nil
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't read system stats: %v\n", err)
		return nil
	}

	parts := strings.Split(line, " ")

	// name, _ := strconv.Atoi(parts[0])
	user, _ := strconv.Atoi(parts[1])
	nice, _ := strconv.Atoi(parts[2])
	system, _ := strconv.Atoi(parts[3])
	idle, _ := strconv.Atoi(parts[4])
	iowait, _ := strconv.Atoi(parts[5])
	// irq, _ := strconv.Atoi(parts[6])
	softirq, _ := strconv.Atoi(parts[7])
	steal, _ := strconv.Atoi(parts[8])
	// guest, _ := strconv.Atoi(parts[9])

	stats := &cpuStats{active: user + system + nice + softirq + steal}
	stats.total = stats.active + idle + iowait

	return stats
}

// MonitorTyping sets the keyboard colors acccording to rate of typing
func MonitorTyping(delay time.Duration, inputEventID string, hotRate float64) {
	if delay == 0 {
		delay = 300 * time.Millisecond
	}

	if inputEventID == "" {
		inputEventID = getInputEventID()
		if inputEventID == "" {
			return
		}
	}

	if hotRate == 0 {
		hotRate = 10.0
	}

	eventpath := "/dev/input/" + inputEventID
	f, err := os.Open(eventpath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't open input events device (%s): %v\n", eventpath, err)
		return
	}

	count := int32(0)
	ColorFileHandler(coldHotColors[0])

	go func() {
		lastIndex := 0
		colorsLen := len(coldHotColors)
		for {
			time.Sleep(delay)
			i := int(atomic.LoadInt32(&count))
			if i == lastIndex {
				continue // no color change needed
			}
			if i >= colorsLen {
				i = colorsLen - 1
			}
			color := coldHotColors[i]
			ColorFileHandler(color)
			if i > 0 {
				atomic.AddInt32(&count, -1)
			}
		}
	}()

	// https://janczer.github.io/work-with-dev-input/
	buf := make([]byte, 24)
	for {
		_, err := f.Read(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't read input events device (%s): %v\n", eventpath, err)
			return
		}

		typ := binary.LittleEndian.Uint16(buf[16:18])
		// code := binary.LittleEndian.Uint16(buf[18:20])

		var value int32
		binary.Read(bytes.NewReader(buf[20:]), binary.LittleEndian, &value)

		// we only care when typ is EV_KEY and value indicates "pressed"
		// https://github.com/torvalds/linux/blob/v5.17/include/uapi/linux/input-event-codes.h#L34-L51
		if typ == 1 && value == 1 {
			// sec := binary.LittleEndian.Uint64(buf[0:8])
			// usec := binary.LittleEndian.Uint64(buf[8:16])
			// ts := time.Unix(int64(sec), int64(usec)*1000)
			atomic.AddInt32(&count, 1)
		}
	}
}

var keyboardEventRE = regexp.MustCompile(`[= ](event\d+)( |$)`)

func getInputEventID() string {
	f, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't open input devices list: %v\n", err)
		return ""
	}
	defer f.Close()

	found := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		switch line[0] {
		case 'N':
			found = strings.Contains(strings.ToLower(line), "keyboard")
		case 'H':
			if found {
				match := keyboardEventRE.FindStringSubmatch(line)
				if match != nil {
					return match[1]
				}
			}
		}
	}

	return ""
}

var pictureURIMonitorRE = regexp.MustCompile(`^\s*picture-uri(?:-dark)?:\s*'([^']+)'\s*$`)
var backgroundProcess *os.Process
var stopRequested = false

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

	backgroundProcess = cmd.Process

	go func() {
		state, err := backgroundProcess.Wait()
		if !stopRequested {
			fmt.Fprintf(os.Stderr, "gnome background monitor has stopped (%v): %v\n", state, err)
			os.Exit(5)
		}
	}()

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

func StopDesktopBackgroundMonitor() {
	if backgroundProcess != nil {
		p := backgroundProcess
		backgroundProcess = nil
		stopRequested = true
		err := p.Kill()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cant' kill gnome background monitor: %v\n", err)
		}
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
			if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
				// we need access to the user's gnome session in order to look up correct setting values
				panic("running as root without user environment: add `-E` when invoking sudo")
			}
		}
	}

	group = "org.gnome.desktop." + group
	args = append(args, action, group, key)
	cmd := exec.Command(cmdName, args...)
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
