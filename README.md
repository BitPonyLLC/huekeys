# sys76-kb
This is cli is still a work-in-progress. The goal is to create a robust tool for managing the RGB keyboard on System76 laptops. The only built-in method System76 provides for changing the colors and brightness of the keyboard is via built-in keys. This cli aims to give users greater control over the keyboard.

Only tested on the Darter, but it should work on other System76 models.

Requires sudo privs to modify the backlight files in `/sys/class/leds/system76`. You may consider adjusting permissions. For example, if your user is in the `adm` group (use `id` to determine your group membership), the following will allow setting of color and brightness:
```
$ ( cd /sys/class/leds/system76*\:\:kbd_backlight && \
    sudo chgrp adm color* brightness && \
    sudo chmod 664 brightness color* )
```

NOTE: this only works until reboot since these devices will be recreated w/ the original permissions.

### usage
```
## help menu
$ sudo sys76-kb

## set color to red
$ sudo sys76-kb set -c red

## set brightness
$ sudo sys76-kb set -b 255

## set color and brightness
$ sudo sys76-kb set -c pink -b 127

## run a infinite rainbow in the background
$ sudo sys76-kb run -p rainbow &

## run a infinite pulse in the background
$ sudo sys76-kb run -p pulse &

```

![alt text][loop]

[loop]: https://github.com/BitPonyLLC/huekeys/blob/master/kb.gif "loop"
