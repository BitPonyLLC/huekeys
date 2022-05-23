package image_matcher

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"

	"github.com/EdlinOrg/prominentcolor"
)

func load(pathname string) (image.Image, error) {
	f, err := os.Open(pathname)
	if err != nil {
		return nil, fmt.Errorf("unable to open %s: %w", pathname, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("unable to decode %s: %w", pathname, err)
	}

	return img, nil
}

func GetDominantColorOf(pathname string) (string, error) {
	img, err := load(pathname)
	if err != nil {
		return "", err
	}

	colors, err := prominentcolor.KmeansWithArgs(prominentcolor.ArgumentNoCropping, img)
	if err != nil {
		return "", fmt.Errorf("unable to extract dominate color: %w", err)
	}

	var best *prominentcolor.ColorItem
	for i, color := range colors {
		if best == nil || color.Cnt > best.Cnt {
			best = &colors[i]
		}
	}

	if best == nil {
		return "", errors.New("no colors found")
	}

	return best.AsString(), nil
}
