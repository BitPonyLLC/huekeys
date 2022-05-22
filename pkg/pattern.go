package keyboard

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// BrightnessPulse continuously dials up and down brightness
func BrightnessPulse(ctx context.Context, delay time.Duration) {
	for {
		for i := 255; i >= 0; i-- {
			s := strconv.Itoa(i)
			BrightnessFileHandler(s)
			if sleep(ctx, delay) {
				return
			}
		}
		for i := 1; i <= 255; i++ {
			s := strconv.Itoa(i)
			BrightnessFileHandler(s)
			if sleep(ctx, delay) {
				return
			}
		}
	}
}

// InfiniteRainbow generates... an infinite rainbow
func InfiniteRainbow(ctx context.Context, delay time.Duration) {
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

	for {
		for i := currentColorOffset; i < len(colors); i++ {
			c := colors[i]
			ColorFileHandler(c)
			if sleep(ctx, delay) {
				return
			}
		}
		currentColorOffset = 0 // only used on first pass
	}
}

// InfinitRandom sets the keyboard colors to random values forever
func InfiniteRandom(ctx context.Context, delay time.Duration) {
	for {
		ColorFileHandler(RandomColor)
		if sleep(ctx, delay) {
			return
		}
	}
}

// MonitorCPU sets the keyboard colors according to CPU utilization
func MonitorCPU(ctx context.Context, delay time.Duration) {
	for {
		previous := getCPUStats()
		if sleep(ctx, delay) {
			return
		}
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
		log.Error().Err(err).Msg("can't open system stats")
		return nil
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	line, err := reader.ReadString('\n')
	if err != nil {
		log.Error().Err(err).Msg("can't read system stats")
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
func MonitorTyping(ctx context.Context, delay time.Duration, inputEventID string, idleCB func(context.Context)) {
	if inputEventID == "" {
		inputEventID = getInputEventID()
		if inputEventID == "" {
			return
		}
	}

	eventpath := "/dev/input/" + inputEventID
	f, err := os.Open(eventpath)
	if err != nil {
		log.Error().Err(err).Str("eventpath", eventpath).Msg("can't open input events device")
		return
	}

	keyPressCount := int32(0)
	ColorFileHandler(coldHotColors[0])

	go setTypingColor(ctx, delay, &keyPressCount, idleCB)

	// https://janczer.github.io/work-with-dev-input/
	buf := make([]byte, 24)
	for {
		_, err := f.Read(buf)
		if err != nil {
			log.Error().Err(err).Str("eventpath", eventpath).Msg("can't read input events device")
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
			atomic.AddInt32(&keyPressCount, 1)
		}
	}
}

func setTypingColor(ctx context.Context, delay time.Duration, keyPressCount *int32, idleCB func(context.Context)) {
	var idleAt *time.Time
	var cancelFunc context.CancelFunc

	lastIndex := 0
	colorsLen := len(coldHotColors)

	for {
		if sleep(ctx, delay) {
			if cancelFunc != nil {
				cancelFunc()
			}
			return
		}

		i := int(atomic.LoadInt32(keyPressCount))
		if i >= colorsLen {
			i = colorsLen - 1
		}

		// don't bother setting the same value
		if i != lastIndex {
			color := coldHotColors[i]
			ColorFileHandler(color)
			lastIndex = i
		}

		if i > 0 {
			idleAt = nil
			if cancelFunc != nil {
				cancelFunc()
				cancelFunc = nil
			}
			atomic.AddInt32(keyPressCount, -1)
			continue
		}

		if idleAt == nil {
			now := time.Now()
			idleAt = &now
			continue
		}

		if cancelFunc != nil {
			continue
		}

		diff := time.Since(*idleAt)
		if diff > 30*time.Second {
			var cancelCtx context.Context
			cancelCtx, cancelFunc = context.WithCancel(ctx)
			go func() { idleCB(cancelCtx) }()
		}
	}
}

var keyboardEventRE = regexp.MustCompile(`[= ](event\d+)( |$)`)

func getInputEventID() string {
	f, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		log.Error().Err(err).Msg("can't open input devices list")
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

func MatchDesktopBackground(ctx context.Context) {
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
		log.Error().Err(err).Str("picture-uri", pictureURIStr).Msg("can't parse")
		return
	}

	setColorFrom(pictureURL.Path)

	cmd := newDesktopSettingCmd("monitor", "background", pictureGroup)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error().Err(err).Msg("can't get stdout of desktop background monitor")
		return
	}

	err = cmd.Start()
	if err != nil {
		log.Error().Err(err).Msg("can't start desktop background monitor")
		return
	}

	backgroundProcess = cmd.Process

	go func() {
		state, err := backgroundProcess.Wait()
		var ev *zerolog.Event
		if stopRequested {
			ev = log.Debug()
		} else {
			ev = log.Error()
		}
		ev.Err(err).Interface("state", state).Msg("desktop background monitor has stopped")
	}()

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			m := pictureURIMonitorRE.FindStringSubmatch(line)
			if m == nil || len(m) < 2 || m[1] == "" {
				log.Warn().Str("line", line).Msg("ignoring unknown content from desktop background monitor")
				continue
			}
			setColorFrom(m[1])
		}
	}()

	<-ctx.Done()
	StopDesktopBackgroundMonitor()
}

func StopDesktopBackgroundMonitor() {
	if backgroundProcess != nil {
		p := backgroundProcess
		log.Debug().Int("pid", p.Pid).Msg("stopping desktop background monitor")
		backgroundProcess = nil
		stopRequested = true
		err := p.Kill()
		if err != nil {
			log.Error().Err(err).Int("pid", p.Pid).Msg("can't kill desktop background monitor")
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
				log.Fatal().Msg("running as root without user environment: add `-E` when invoking sudo")
			}
		}
	}

	group = "org.gnome.desktop." + group
	args = append(args, action, group, key)
	cmd := exec.Command(cmdName, args...)
	return cmd
}

func getDesktopSetting(group, key string) (string, error) {
	// TODO: consider using D-Bus directly instead of gsettings...
	val, err := newDesktopSettingCmd("get", group, key).Output()
	if err != nil {
		log.Error().Err(err).Str("group", group).Str("key", key).Msg("can't get setting value")
		return "", err
	}

	val = bytes.TrimFunc(val, func(r rune) bool { return unicode.IsSpace(r) || r == '\'' })
	return string(val), nil
}

func setColorFrom(u string) {
	pictureURL, err := url.Parse(u)
	if err != nil {
		log.Error().Err(err).Str("uri", u).Msg("can't parse picture uri")
		return
	}

	color, err := image_matcher.GetDominantColorOf(pictureURL.Path)
	if err != nil {
		log.Error().Err(err).Msg("can't determine dominant color")
		return
	}

	ColorFileHandler(color)
}

func sleep(ctx context.Context, delay time.Duration) bool {
	wake := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		wake.Stop()
		return true
	case <-wake.C:
		return false
	}
}
