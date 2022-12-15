package generator

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkGenerateThumbnails(b *testing.B) {
	config := Config{
		baseName:              "benchmark",
		bgImageFilePath:       filepath.Join("testdata", "images", "bgimage.png"),
		destPath:              filepath.Join("testdata", "output"),
		fontBorderAlphaThresh: 250,
		fontBorderColor:       color.NRGBA{R:255, G:255, B:255, A:255},
		fontBorderWidth:       3,
		fontColor:             &image.Uniform{C: color.NRGBA{R:0, G:0, B:0, A:255}},
		fontFilePath:          filepath.Join("testdata", "fonts", "tahomabd.ttf"),
		fontSize:              25,
		numDigits:             3,
		numPosX:               986,
		numPosY:               625,
		numEnd:                100,
		numStart:              1,
		textImgHeight:         1080,
		textImgWidth:          1920,
	}
	setConf(config)
	err := configFreetype(config.fontFilePath)
	assert.NoError(b, err)
	err = importBackground(config.bgImageFilePath)
	assert.NoError(b, err)
	err = generateThumbnails()
	assert.NoError(b, err)
}