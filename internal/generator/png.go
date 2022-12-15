package generator

import (
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	borders "github.com/SkyMack/image-add-borders"
	"github.com/golang/freetype"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	baseNameFlagName        = "base-name"
	bgImageFlagName         = "bg-image"
	destPathFlagName        = "output-dest"
	fontBorderAlphaThreshold = "font-border-alpha-thresh"
	fontBorderColorFlagName = "font-border-color"
	fontBorderWidthFlagName = "font-border-width"
	fontColorFlagName       = "font-color"
	fontSizeFlagName        = "font-size"
	seqEndFlagName          = "seq-end"
	seqNumDigitsFlagName    = "seq-num-digits"
	seqNumPosXFlagname      = "seq-num-pos-x"
	seqNumPosYFlagname      = "seq-num-pos-y"
	seqStartFlagName        = "seq-start"
	textLayerHeightFlagName = "text-layer-height"
	textLayerWidthFlagName  = "text-layer-width"
)

var (
	bgImage *image.NRGBA
	conf    Config
	ftCtx   = freetype.NewContext()
)

type thumbnail struct {
	image        *image.NRGBA
	paddedNumber string
	number       int
}

// Config is used to store the configuration options for the thumbnail generator
type Config struct {
	baseName              string
	bgImageFilePath       string
	destPath              string
	fontBorderAlphaThresh uint8
	fontBorderColor       color.NRGBA
	fontBorderWidth       int
	fontColor             *image.Uniform
	fontFilePath          string
	fontSize              float64
	numDigits             int
	numPosX               int
	numPosY               int
	numEnd                int
	numStart              int
	textImgHeight         int
	textImgWidth          int
}

func addGeneratePngFlags(cmdFlags *pflag.FlagSet) {
	genPngFlags := &pflag.FlagSet{}

	genPngFlags.String(baseNameFlagName, "", "The base name for the image files (required)")
	genPngFlags.String(bgImageFlagName, "", "Full path to the background image (required)")
	genPngFlags.String(destPathFlagName, "", "Full path to the output destination (required)")
	genPngFlags.Uint8(fontBorderAlphaThreshold, 0, "The alpha value at which we consider a pixel to be empty/convert to a border pixel")
	genPngFlags.String(fontBorderColorFlagName, "FFFFFF", "Sequence number outline color (6 character RGB hex code)")
	genPngFlags.Int(fontBorderWidthFlagName, 2, "Sequence number outline thickness (in pixels)")
	genPngFlags.String(fontColorFlagName, "000000", "Sequence number text color (6 character RGB hex code)")
	genPngFlags.Float64(fontSizeFlagName, 30, "Font size in points")
	genPngFlags.Int(seqNumDigitsFlagName, 2, "Number of fixed places in the generated sequence number (ie. how many 0s to pad single digits with)")
	genPngFlags.Int(seqNumPosXFlagname, 975, "X coordinate the sequence number will be drawn at")
	genPngFlags.Int(seqNumPosYFlagname, 600, "Y coordinate the sequence number will be drawn at")
	genPngFlags.Int(seqStartFlagName, 1, "Number to start the sequence with")
	genPngFlags.Int(seqEndFlagName, 10, "Number to end the sequence on")
	genPngFlags.Int(textLayerHeightFlagName, 1080, "Height of the temporary image the text is drawn onto; may need to be increased when processing very large images")
	genPngFlags.Int(textLayerWidthFlagName, 1920, "Width of the temporary image the text is drawn onto; may need to be increased when processing very large images")

	cmdFlags.AddFlagSet(genPngFlags)
}

func markGeneratorPngRequiredFlags(cmd *cobra.Command) {
	cmd.MarkFlagRequired(baseNameFlagName)
	cmd.MarkFlagRequired(bgImageFlagName)
	cmd.MarkFlagRequired(destPathFlagName)
}

// AddCmdGeneratePng adds the generatepng subcommand to a cobra.Command
func AddCmdGeneratePng(rootCmd *cobra.Command) {
	generatePngCmd := &cobra.Command{
		Use:   "generatepng",
		Short: "generate thumbnails in PNG format",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := createConfigFromFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := checkConfig(conf); err != nil {
				return err
			}
			if err := configFreetype(conf.fontFilePath); err != nil {
				return err
			}
			if err := importBackground(conf.bgImageFilePath); err != nil {
				return err
			}

			if err := generateThumbnails(); err != nil {
				return err
			}

			return nil
		},
	}

	addGeneratePngFlags(generatePngCmd.Flags())
	markGeneratorPngRequiredFlags(generatePngCmd)

	rootCmd.AddCommand(generatePngCmd)
}

var errInvalidFormat = errors.New("invalid RGB hex code")

// ParseHexColor converts a 6 rune string of hexidecimal characters into a color.NRGBA
func ParseHexColor(s string) (colr color.NRGBA, err error) {
	colr.A = 0xff
	if len([]rune(s)) != 6 {
		return colr, errInvalidFormat
	}
	colrBytes, err := hex.DecodeString(s)
	if err != nil {
		return colr, err
	}

	colr.R = colrBytes[0]
	colr.G = colrBytes[1]
	colr.B = colrBytes[2]
	return
}

func createConfigFromFlags(flags *pflag.FlagSet) error {
	baseName, err := flags.GetString(baseNameFlagName)
	if err != nil {
		return err
	}
	bgImageFilePath, err := flags.GetString(bgImageFlagName)
	if err != nil {
		return err
	}
	destPath, err := flags.GetString(destPathFlagName)
	if err != nil {
		return err
	}
	fontBorderAlphaThreshold, err := flags.GetUint8(fontBorderAlphaThreshold)
	if err != nil {
		return err
	}
	fontBorderColorStr, err := flags.GetString(fontBorderColorFlagName)
	if err != nil {
		return nil
	}
	fontBorderWidth, err := flags.GetInt(fontBorderWidthFlagName)
	if err != nil {
		return nil
	}
	fontColorStr, err := flags.GetString(fontColorFlagName)
	if err != nil {
		return err
	}
	fontSize, err := flags.GetFloat64(fontSizeFlagName)
	if err != nil {
		return err
	}
	numPlaces, err := flags.GetInt(seqNumDigitsFlagName)
	if err != nil {
		return err
	}
	numPosX, err := flags.GetInt(seqNumPosXFlagname)
	if err != nil {
		return err
	}
	numPosY, err := flags.GetInt(seqNumPosYFlagname)
	if err != nil {
		return err
	}
	seqStart, err := flags.GetInt(seqStartFlagName)
	if err != nil {
		return err
	}
	seqEnd, err := flags.GetInt(seqEndFlagName)
	if err != nil {
		return err
	}
	textLayerHeight, err := flags.GetInt(textLayerHeightFlagName)
	if err != nil {
		return err
	}
	textLayerWidth, err := flags.GetInt(textLayerWidthFlagName)
	if err != nil {
		return err
	}

	fontBorderColor, err := ParseHexColor(fontBorderColorStr)
	if err != nil {
		return err
	}
	fontColor, err := ParseHexColor(fontColorStr)
	if err != nil {
		return err
	}
	//fontColorRHex, fontColorGHex, fontColorBHex :=
	config := Config{
		baseName:              baseName,
		bgImageFilePath:       bgImageFilePath,
		destPath:              destPath,
		fontBorderAlphaThresh: fontBorderAlphaThreshold,
		fontBorderColor:       fontBorderColor,
		fontBorderWidth:       fontBorderWidth,
		fontColor:             &image.Uniform{C: fontColor},
		fontFilePath:          filepath.Join("assets", "fonts", "tahomabd.ttf"),
		fontSize:              fontSize,
		numDigits:             numPlaces,
		numPosX:               numPosX,
		numPosY:               numPosY,
		numEnd:                seqEnd,
		numStart:              seqStart,
		textImgHeight:         textLayerHeight,
		textImgWidth:          textLayerWidth,
	}

	setConf(config)
	return nil
}

func checkConfig(config Config) error {
	var result error

	if config.numStart < 0 || config.numEnd <= 0 {
		result = multierror.Append(result, fmt.Errorf("invalid sequence: start must be a positive number"))
	}
	if config.numStart >= config.numEnd {
		result = multierror.Append(result, fmt.Errorf("invalid sequence: start number is the same as or after the end number"))
	}
	if config.numDigits < 0 {
		result = multierror.Append(result, fmt.Errorf("invalid numFixedPlaces: must be a positive number"))
	}
	return result
}

func setConf(config Config) {
	conf = config
	log.WithFields(log.Fields{
		"conf": fmt.Sprintf("%v", conf),
	}).Debug("config")
}

// generateThumbnails renders and saves thumbnails comprised of the given background image and a sequence number
func generateThumbnails() error {
	for i := conf.numStart; i <= conf.numEnd; i++ {
		thumbNail := thumbnail{
			image: &*bgImage,
			number: i,
		}
		if err := thumbNail.render(); err != nil {
			return err
		}
		if err := thumbNail.export(conf.destPath); err != nil {
			return err
		}
	}

	return nil
}

func configFreetype(fontFilePath string) error {
	fontBytes, err := os.ReadFile(fontFilePath)
	if err != nil {
		return err
	}
	font, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return err
	}

	ftCtx.SetFont(font)
	ftCtx.SetDPI(300)
	ftCtx.SetFontSize(conf.fontSize)
	ftCtx.SetSrc(conf.fontColor)

	return nil
}

func importBackground(filepath string) error {
	fileData, err := os.Open(filepath)
	if err != nil {
		return err
	}

	imageData, _, err := image.Decode(fileData)
	if err != nil {
		return err
	}
	bgImage = image.NewNRGBA(imageData.Bounds())
	draw.Draw(bgImage, bgImage.Bounds(), imageData, image.Point{}, draw.Src)
	return nil
}

func (thumb *thumbnail) setPaddedNumberFromNumber() {
	log.WithFields(log.Fields{
		"conf.numDigits": conf.numDigits,
		"thumb.number":   thumb.number,
	}).Trace("entered setPaddedNumberFromNumber")

	raw := strconv.Itoa(thumb.number)
	rawCharCount := strings.Count(raw, "") - 1
	if conf.numDigits <= 1 || rawCharCount >= conf.numDigits {
		log.WithFields(log.Fields{
			"thumb.number.raw_char_count": rawCharCount,
		}).Trace("exiting setPaddedNumberFromNumber using unpadded value")
		thumb.paddedNumber = raw
	}

	final := raw
	for i := 1; i <= conf.numDigits-rawCharCount; i++ {
		final = fmt.Sprintf("0%s", final)
	}

	log.WithFields(log.Fields{
		"conf.numDigits":              conf.numDigits,
		"thumb.number":                thumb.number,
		"thumb.number.padded":         final,
		"thumb.number.raw_char_count": rawCharCount,
	}).Debugf("setPaddedNumberFromNumber 0 padded result")
	thumb.paddedNumber = final
}

// render creates the image and the number overlay for the thumbnail
func (thumb *thumbnail) render() error {
	// generate padded number string
	thumb.setPaddedNumberFromNumber()

	// create a new, blank image
	thumb.image = image.NewNRGBA(bgImage.Bounds())

	// draw the background image onto the blank image
	draw.Draw(thumb.image, bgImage.Bounds(), bgImage, image.Point{}, draw.Over)

	// create a temp image to draw the text onto
	textImg := image.NewNRGBA(image.Rect(0, 0, conf.textImgWidth, conf.textImgHeight))

	// setup freetype
	ftCtx.SetDst(textImg)
	ftCtx.SetClip(textImg.Bounds())

	// draw the sequence number onto the temp image
	//numPosition := freetype.Pt(conf.numPosX, conf.numPosY)
	numPosition := freetype.Pt(10+conf.fontBorderWidth, conf.textImgHeight-(10+conf.fontBorderWidth))
	if _, err := ftCtx.DrawString(fmt.Sprintf("#%s", thumb.paddedNumber), numPosition); err != nil {
		return nil
	}
	// add an outline to the text
	borders.AddBorders(textImg, conf.fontBorderColor, conf.fontBorderWidth, conf.fontBorderAlphaThresh)
	borderColorSoft := conf.fontBorderColor
	borderColorSoft.A = 150
	borderColorSofter := conf.fontBorderColor
	borderColorSofter.A = 65
	borders.AddBorders(textImg, borderColorSoft, 1, conf.fontBorderAlphaThresh)
	borders.AddBorders(textImg, borderColorSofter, 1, 149)
	if _, err := ftCtx.DrawString(fmt.Sprintf("#%s", thumb.paddedNumber), numPosition); err != nil {
		return nil
	}
	textRect := calcMinRect(textImg)

	if log.GetLevel() == log.DebugLevel {
		fileName := fmt.Sprintf("thumbnail_%s_%s_debug_textlayer.png", conf.baseName, thumb.paddedNumber)
		destFile, err := os.Create(filepath.Join(conf.destPath, fileName))
		if err != nil {
			return err
		}
		png.Encode(destFile, textImg)
	}

	// draw the outlined text onto the thumbnail
	textRectAbs := image.Rectangle{
		Min: image.Point{0,0},
		Max: textRect.Size(),
	}
	destRect := textRectAbs.Bounds().Add(image.Point{X: conf.numPosX, Y:conf.numPosY})
	log.WithFields(log.Fields{
		"destrect.min.x": destRect.Min.X,
		"destrect.min.y": destRect.Min.Y,
		"textbox.min.x": textRect.Min.X,
		"textbox.min.y": textRect.Min.Y,
	}).Debug("overlay settings")
	draw.Draw(thumb.image, destRect, textImg, textRect.Min, draw.Over)
	//draw.Draw(thumb.image, textRect.Bounds().Add(image.Point{X: conf.numPosX, Y: conf.numPosY}), textImg, textRect.Min, draw.Over)
	//draw.Draw(thumb.image, textImg.Bounds().Add(image.Point{X: conf.numPosX, Y: conf.numPosY}), textImg, image.Point{}, draw.Over)
	//draw.Draw(thumb.image, textImg.Bounds().Add(image.Point{X: conf.numPosX, Y: conf.numPosY}), textImg, textRect.Min, draw.Over)
	return nil
}

func (thumb *thumbnail) export(destPath string) error {
	fileName := fmt.Sprintf("thumbnail_%s_%s.png", conf.baseName, thumb.paddedNumber)
	destFile, err := os.Create(filepath.Join(destPath, fileName))
	if err != nil {
		return err
	}
	png.Encode(destFile, thumb.image)
	return nil
}

func calcMinRect(img *image.NRGBA) image.Rectangle {
	var minX, minY, maxX, maxY int
	var valueFound bool

	imgHeight := img.Bounds().Dy()
	imgWidth := img.Bounds().Dx()

	valueFound = false
	for y := 0; y <= imgHeight; y++ {
		for x := 0; x <= imgWidth; x++ {
			if !borders.IsEmptyPixel(img, image.Point{X: x, Y: y}) {
				minY = y
				valueFound = true
				break
			}
		}
		if valueFound {
			break
		}
	}
	valueFound = false
	for y := imgHeight; y >= 0; y-- {
		for x := imgWidth; x >= 0; x-- {
			if !borders.IsEmptyPixel(img, image.Point{X: x, Y: y}) {
				maxY = y
				valueFound = true
				break
			}
		}
		if valueFound {
			break
		}
	}

	valueFound = false
	for x := 0; x <= imgWidth; x++ {
		for y := 0; y <= imgHeight; y++ {
			if !borders.IsEmptyPixel(img, image.Point{X: x, Y: y}) {
				minX = x
				valueFound = true
				break
			}
		}
		if valueFound {
			break
		}
	}
	valueFound = false
	for x := imgWidth; x >= 0; x-- {
		for y := imgHeight; y >= 0; y-- {
			if !borders.IsEmptyPixel(img, image.Point{X: x, Y: y}) {
				maxX = x
				valueFound = true
				break
			}
		}
		if valueFound {
			break
		}
	}

	//comment
	log.WithFields(log.Fields{
		"min.coord": fmt.Sprintf("%d, %d", minX, minY),
		"min.x":     minX,
		"min.y":     minY,
		"max.coord": fmt.Sprintf("%d, %d", maxX, maxY),
		"max.x":     maxX,
		"max.y":     maxY,
	}).Debug("calcMinRect")
	return image.Rect(minX-1, minY-1, maxX+1, maxY+1)
}