package robot3d

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// ParseColor reads "#rrggbb".
func ParseColor(s string) (color.Color, error) {
	var r, g, b uint8
	if _, err := fmt.Sscanf(strings.TrimPrefix(strings.TrimSpace(s), "#"), "%02x%02x%02x", &r, &g, &b); err != nil {
		return nil, fmt.Errorf("colour %q: want #rrggbb", s)
	}
	return color.RGBA{r, g, b, 255}, nil
}

// ParseLEDs reads the robot's 12 LEDs as comma separated "#rrggbb" (left 0-5, right 6-11, as you face
// the screen; 0 and 11 at the front; empty: off), or "#rrggbb*12" for all of them.
func ParseLEDs(s string) ([12]color.Color, error) {
	var leds [12]color.Color
	if strings.TrimSpace(s) == "" {
		return leds, nil
	}
	list := strings.Split(s, ",")
	if c, n, ok := strings.Cut(s, "*"); ok {
		k, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil || k < 1 || k > 12 {
			return leds, fmt.Errorf("leds %q: want #rrggbb*N, N 1..12", s)
		}
		list = list[:0]
		for range k {
			list = append(list, c)
		}
	}
	if len(list) > 12 {
		return leds, fmt.Errorf("leds %q: 12 at most", s)
	}
	for i, c := range list {
		if strings.TrimSpace(c) == "" {
			continue
		}
		v, err := ParseColor(c)
		if err != nil {
			return leds, err
		}
		leds[i] = v
	}
	return leds, nil
}
