# sys76-kb
This is app is still a work-in-progress. The goal is to create a robust tool for managing the RGB keyboard on System76 laptops. The only method System76 provides for changing the colors and brightness of the keyboard is via built-in keys. This app aims to give users greater control over the keyboard.

Currently, it can control the brightness, and rotate the keyboard backlight through a RGB rainbow. Only tested on the Darter, but it should work on other System76 models.

Requires sudo privs to modify the backlight files in `/sys/class/leds/system76`

### usage
```
## help menu
$ sudo sys76-kb

## run a infinite rainbow in the background
$ sudo sys76-kb run &

## set brightness (use 0 - 255)
$ sudo sys76-kb brightness -l 100
```

![alt text][loop]

[loop]: https://github.com/bambash/sys76-kb/blob/master/kb.gif "loop"

### future plans
- more cli functionality
- custom hex values
- pre-built RGB patterns
