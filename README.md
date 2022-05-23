# huekeys

Huekeys is a fun application that makes it easy to adjust your System76 keyboard colors and brightness. In addition to the simple ability to set and get the color and brightness, it also provides several patterns that you can run indefinitely, to really make your keyboard pop!

### Installation

Requires sudo privs to modify the backlight files in `/sys/class/leds/system76`. You may consider adjusting permissions. For example, if your user is in the `adm` group (use `id` to determine your group membership), the following will allow setting of color and brightness:
```
$ ( cd /sys/class/leds/system76*\:\:kbd_backlight && \
    sudo chgrp adm color* brightness && \
    sudo chmod 664 brightness color* )
```

NOTE: this only works until reboot since these devices will be recreated w/ the original permissions.

### Usage

```
## help menu
$ sudo huekeys

## set color to red
$ sudo huekeys set -c red

## set brightness
$ sudo huekeys set -b 255

## set color and brightness
$ sudo huekeys set -c pink -b 127

## run a infinite rainbow in the background
$ sudo huekeys run rainbow &

## run a infinite pulse in the background
$ sudo huekeys run pulse &
```

![alt text][loop]

[loop]: https://github.com/BitPonyLLC/huekeys/blob/master/kb.gif "loop"

## Attribution

This project was originally produced as https://github.com/bambash/sys76-kb. Though it's signifcantly different as `huekeys`, a huge thanks goes out to bambash's original as an excellent starting point!
