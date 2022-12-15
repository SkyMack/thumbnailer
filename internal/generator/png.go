package generator

import (
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SkyMack/imgutil"
	"github.com/golang/freetype/truetype"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const (
	baseNameFlagName         = "base-name"
	bgImageFlagName          = "bg-image"
	destPathFlagName         = "output-dest"
	fontBorderAlphaThreshold = "font-border-alpha-thresh"
	fontBorderColorFlagName  = "font-border-color"
	fontBorderWidthFlagName  = "font-border-width"
	fontColorFlagName        = "font-color"
	fontSizeFlagName         = "font-size"
	seqEndFlagName           = "seq-end"
	seqNumDigitsFlagName     = "seq-num-digits"
	seqNumPosXFlagname       = "seq-num-pos-x"
	seqNumPosYFlagname       = "seq-num-pos-y"
	seqStartFlagName         = "seq-start"
	textLayerHeightFlagName  = "text-layer-height"
	textLayerWidthFlagName   = "text-layer-width"

	fontDPI = 300
)

var (
	bgImage    *image.NRGBA
	conf       Config
	debug      bool
	parsedFont *truetype.Font
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

func init() {
	if log.GetLevel() == log.DebugLevel {
		debug = true
	}
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
			if err := importBackground(conf.bgImageFilePath); err != nil {
				return err
			}
			if err := parseFontFile(); err != nil {
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
		"conf": fmt.Sprintf("%+v", conf),
	}).Debug("running config")
}

func parseFontFile() error {
	fontBytes, err := os.ReadFile(conf.fontFilePath)
	if err != nil {
		return err
	}
	parsedFont, err = truetype.Parse(fontBytes)
	if err != nil {
		return err
	}

	return nil
}

// generateThumbnails renders and saves thumbnails comprised of the given background image and a sequence number
func generateThumbnails() error {
	for i := conf.numStart; i <= conf.numEnd; i++ {
		thumbNail := thumbnail{
			image:  &*bgImage,
			number: i,
		}
		if err := thumbNail.render(); err != nil {
			return err
		}
		if err := thumbNail.export(); err != nil {
			return err
		}
	}

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
	raw := strconv.Itoa(thumb.number)
	rawCharCount := strings.Count(raw, "") - 1

	if conf.numDigits <= 1 || rawCharCount >= conf.numDigits {
		// No padding required (number is longer than or equal to the number of places, so no leading 0s needed)
		thumb.paddedNumber = raw
	}

	paddedNum := raw
	for i := 1; i <= conf.numDigits-rawCharCount; i++ {
		paddedNum = fmt.Sprintf("0%s", paddedNum)
	}

	log.WithFields(log.Fields{
		"conf.numDigits":              conf.numDigits,
		"thumb.number":                thumb.number,
		"thumb.number.padded":         paddedNum,
		"thumb.number.raw_char_count": rawCharCount,
	}).Debug("setPaddedNumberFromNumber result")
	thumb.paddedNumber = paddedNum
}

// render creates the image and the number overlay for the thumbnail
func (thumb *thumbnail) render() error {
	// generate padded number string
	thumb.setPaddedNumberFromNumber()

	// create a new, blank image
	thumb.image = image.NewNRGBA(bgImage.Bounds())

	// draw the background image onto the blank
	draw.Draw(thumb.image, bgImage.Bounds(), bgImage, image.Point{}, draw.Over)

	borderColorSoft := conf.fontBorderColor
	borderColorSoft.A = 150
	borderColorSofter := conf.fontBorderColor
	borderColorSofter.A = 65

	// create a temp image to draw the text onto
	textImg := image.NewNRGBA(image.Rect(0, 0, conf.textImgWidth, conf.textImgHeight))

	// calc Y level to place the font Drawer dot at, given the font size and DPI
	y := int(math.Ceil(conf.fontSize * fontDPI / 72))
	mathDot := fixed.Point26_6{
		X: fixed.I(0 + 2 + conf.fontBorderWidth),
		Y: fixed.I(y + ((2 + conf.fontBorderWidth) * 2)),
	}
	textDrawer := &font.Drawer{
		Dst: textImg,
		Src: conf.fontColor,
		Face: truetype.NewFace(ft, &truetype.Options{
			Size:    conf.fontSize,
			DPI:     fontDPI,
			Hinting: font.HintingFull,
		}),
		Dot: mathDot,
	}
	text := fmt.Sprintf("#%v", thumb.paddedNumber)
	// draw the sequence number onto the temp image
	textDrawer.DrawString(text)
	imgutil.AddBorders(textImg, conf.fontBorderColor, conf.fontBorderWidth, conf.fontBorderAlphaThresh)
	textDrawer.Dot = mathDot
	textDrawer.DrawString(text)
	imgutil.AddBorders(textImg, borderColorSoft, 1, conf.fontBorderAlphaThresh)
	imgutil.AddBorders(textImg, borderColorSofter, 1, 149)
	if debug {
		fileName := fmt.Sprintf("thumbnail_%s_%s_debug_textlayer.png", conf.baseName, thumb.paddedNumber)
		filePath := filepath.Join(conf.destPath, fileName)
		if err := savePNG(textImg, filePath); err != nil {
			return err
		}
	}

	textRect := imgutil.OccupiedAreaRect(textImg)
	textRectAbs := image.Rectangle{
		Min: image.Point{0, 0},
		Max: textRect.Size(),
	}
	//destRect := textRectAbs.Bounds().Add(image.Point{X: conf.numPosX, Y:conf.numPosY})
	calcX := thumb.image.Bounds().Dx() - textRectAbs.Bounds().Dx() - 25
	calcY := thumb.image.Bounds().Dy() - textRectAbs.Bounds().Dy() - 25
	destRect := textRectAbs.Bounds().Add(image.Point{X: calcX, Y: calcY})

	draw.Draw(thumb.image, destRect, textImg, textRect.Min, draw.Over)
	return nil
}

func (thumb *thumbnail) export() error {
	fileName := fmt.Sprintf("thumbnail_%s_%s.png", conf.baseName, thumb.paddedNumber)
	filePath := filepath.Join(conf.destPath, fileName)
	if err := savePNG(thumb.image, filePath); err != nil {
		return err
	}
	return nil
}

func savePNG(img image.Image, destFile string) error {
	destFh, err := os.Create(destFile)
	defer destFh.Close()
	if err != nil {
		return err
	}
	if err := png.Encode(destFh, img); err != nil {
		return err
	}
	return nil

}
