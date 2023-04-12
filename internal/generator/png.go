package generator

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SkyMack/imgutils"
	"github.com/golang/freetype/truetype"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/tiff"
)

const (
	flagNameBaseName            = "base-name"
	flagNameBgImage             = "bg-baseImage"
	flagNameDestPath            = "output-dest"
	flagNameFontBorderColor     = "font-border-color"
	flagNameFontBorderWidth     = "font-border-width"
	flagNameFontColor           = "font-color"
	flagNameFontSize            = "font-size"
	flagNameSeqEnd              = "seq-end"
	flagNameSeqNumDigits        = "seq-num-digits"
	flagNameSeqNumPosX          = "seq-num-pos-x"
	flagNameSeqNumPosY          = "seq-num-pos-y"
	flagNameSeqStart            = "seq-start"
	flagNameStillFilenameExt    = "still-filename-ext"
	flagNameStillFilenamePrefix = "still-filename-prefix"
	flagNameStillSrcPath        = "still-src"
	flagNameTextLayerHeight     = "text-layer-height"
	flagNameTextLayerWidth      = "text-layer-width"
	flagNameTitleOverlayPath    = "title-overlay-img"

	fontBorderAlphaThreshold = "font-border-alpha-thresh"
	fontDPI                  = 300

	imageFinalHeight = 720
	imageFinalWidth  = 1280
)

var (
	bgImage    *image.NRGBA
	conf       Config
	debug      bool
	parsedFont *truetype.Font
)

func init() {
	conf = Config{
		dynamic: ConfigDynamic{},
		static:  ConfigStatic{},
	}
}

type thumbnail struct {
	baseImage       *image.NRGBA
	image           *image.NRGBA
	paddedSeqNumber string
	seqNumber       int
	titleImage      *image.NRGBA
}

// Config is used to store the persistent configuration options for the thumbnail generator
type Config struct {
	baseName              string
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

	dynamic ConfigDynamic
	static  ConfigStatic
}

// ConfigStatic stores the configuration options related to generating static thumbnails
type ConfigStatic struct {
	bgImageFilePath string
}

// ConfigDynamic stores the configuration options related to generating dynamic thumbnails
type ConfigDynamic struct {
	stillFilenameExt    string
	stillFilenamePrefix string
	stillSourceDirPath  string
	titleImageFilePath  string
}

func init() {
	_, debugVarSet := os.LookupEnv("DEBUG")
	if log.GetLevel() == log.DebugLevel || debugVarSet {
		debug = true
	}
}

func addPngPersistentFlags(flags *pflag.FlagSet) {
	pngFlags := &pflag.FlagSet{}

	pngFlags.String(flagNameBaseName, "", "The base name for the baseImage files (required)")
	pngFlags.String(flagNameDestPath, "", "Full path to the output destination (required)")
	pngFlags.Uint8(fontBorderAlphaThreshold, 0, "The alpha value at which we consider a pixel to be empty/convert to a border pixel")
	pngFlags.String(flagNameFontBorderColor, "FFFFFF", "Sequence seqNumber outline color (6 character RGB hex code)")
	pngFlags.Int(flagNameFontBorderWidth, 2, "Sequence seqNumber outline thickness (in pixels)")
	pngFlags.String(flagNameFontColor, "000000", "Sequence seqNumber text color (6 character RGB hex code)")
	pngFlags.Float64(flagNameFontSize, 30, "Font size in points")
	pngFlags.Int(flagNameSeqNumDigits, 2, "Number of fixed places in the generated sequence seqNumber (ie. how many 0s to pad single digits with)")
	pngFlags.Int(flagNameSeqNumPosX, 975, "X coordinate the sequence seqNumber will be drawn at")
	pngFlags.Int(flagNameSeqNumPosY, 600, "Y coordinate the sequence seqNumber will be drawn at")
	pngFlags.Int(flagNameSeqStart, 1, "Number to start the sequence with")
	pngFlags.Int(flagNameSeqEnd, 10, "Number to end the sequence on")
	pngFlags.Int(flagNameTextLayerHeight, 1080, "Height of the temporary baseImage the text is drawn onto; may need to be increased when processing very large images")
	pngFlags.Int(flagNameTextLayerWidth, 1920, "Width of the temporary baseImage the text is drawn onto; may need to be increased when processing very large images")

	flags.AddFlagSet(pngFlags)
}

func markPngRequiredFlags(cmd *cobra.Command) error {
	if err := cmd.MarkPersistentFlagRequired(flagNameBaseName); err != nil {
		return err
	}
	if err := cmd.MarkPersistentFlagRequired(flagNameDestPath); err != nil {
		return err
	}

	return nil
}

// AddCmdPng adds the generatepng subcommand to a cobra.Command
func AddCmdPng(parentCmd *cobra.Command) {
	pngCmd := &cobra.Command{
		Use:   "png",
		Short: "generate thumbnails in PNG format",
	}
	addPngPersistentFlags(pngCmd.PersistentFlags())
	if err := markPngRequiredFlags(pngCmd); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("unable to mark required flags")
		os.Exit(1)
	}

	addCmdPngDynamic(pngCmd)
	addCmdPngStatic(pngCmd)

	parentCmd.AddCommand(pngCmd)
}

func addPngStaticFlags(flags *pflag.FlagSet) {
	pngStaticFlags := &pflag.FlagSet{}
	pngStaticFlags.String(flagNameBgImage, "", "Full path to the background baseImage (required)")

	flags.AddFlagSet(pngStaticFlags)
}

func addCmdPngStatic(parentCmd *cobra.Command) {
	pngStaticCmd := &cobra.Command{
		Use:   "static",
		Short: "static baseImage composition (the only difference between thumbnails is the overlayed sequence seqNumber)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := conf.setPersistentConfigFromFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := conf.setStaticConfigFromFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := conf.validate(); err != nil {
				return err
			}
			if err := importBackground(conf.static.bgImageFilePath); err != nil {
				return err
			}
			if err := parseFontFile(); err != nil {
				return err
			}

			if err := generateStaticThumbnails(); err != nil {
				return err
			}

			return nil
		},
	}
	addPngStaticFlags(pngStaticCmd.Flags())

	parentCmd.AddCommand(pngStaticCmd)
}

func addPngDynamicFlags(flags *pflag.FlagSet) {
	pngDynamicFlags := &pflag.FlagSet{}
	pngDynamicFlags.String(flagNameStillFilenameExt, "still", "Filename extension on all still baseImage files")
	pngDynamicFlags.String(flagNameStillFilenamePrefix, "E", "Filename prefix on all still baseImage files")
	pngDynamicFlags.String(flagNameStillSrcPath, "", "Source directory containing all still baseImage files")
	pngDynamicFlags.String(flagNameTitleOverlayPath, "", "Full path to the title overlay baseImage")

	flags.AddFlagSet(pngDynamicFlags)
}

func addCmdPngDynamic(parentCmd *cobra.Command) {
	pngDynamicCmd := &cobra.Command{
		Use:   "dynamic",
		Short: "dynamic baseImage composition (unique primary baseImage per thumbnail)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := conf.setPersistentConfigFromFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := conf.setDynamicConfigFromFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := conf.validate(); err != nil {
				return err
			}
			if err := parseFontFile(); err != nil {
				return err
			}

			if err := generateDynamicThumbnails(); err != nil {
				return err
			}

			return nil
		},
	}
	addPngDynamicFlags(pngDynamicCmd.Flags())

	parentCmd.AddCommand(pngDynamicCmd)
}

func (c *Config) setPersistentConfigFromFlags(flags *pflag.FlagSet) error {
	baseName, err := flags.GetString(flagNameBaseName)
	if err != nil {
		return err
	}
	destPath, err := flags.GetString(flagNameDestPath)
	if err != nil {
		return err
	}
	fontBorderAlphaThreshold, err := flags.GetUint8(fontBorderAlphaThreshold)
	if err != nil {
		return err
	}
	fontBorderColorStr, err := flags.GetString(flagNameFontBorderColor)
	if err != nil {
		return nil
	}
	fontBorderWidth, err := flags.GetInt(flagNameFontBorderWidth)
	if err != nil {
		return nil
	}
	fontColorStr, err := flags.GetString(flagNameFontColor)
	if err != nil {
		return err
	}
	fontSize, err := flags.GetFloat64(flagNameFontSize)
	if err != nil {
		return err
	}
	numPlaces, err := flags.GetInt(flagNameSeqNumDigits)
	if err != nil {
		return err
	}
	numPosX, err := flags.GetInt(flagNameSeqNumPosX)
	if err != nil {
		return err
	}
	numPosY, err := flags.GetInt(flagNameSeqNumPosY)
	if err != nil {
		return err
	}
	seqStart, err := flags.GetInt(flagNameSeqStart)
	if err != nil {
		return err
	}
	seqEnd, err := flags.GetInt(flagNameSeqEnd)
	if err != nil {
		return err
	}
	textLayerHeight, err := flags.GetInt(flagNameTextLayerHeight)
	if err != nil {
		return err
	}
	textLayerWidth, err := flags.GetInt(flagNameTextLayerWidth)
	if err != nil {
		return err
	}

	fontBorderColor, err := imgutils.ParseHexColor(fontBorderColorStr)
	if err != nil {
		return err
	}
	fontColor, err := imgutils.ParseHexColor(fontColorStr)
	if err != nil {
		return err
	}

	c.baseName = baseName
	c.destPath = destPath
	c.fontBorderAlphaThresh = fontBorderAlphaThreshold
	c.fontBorderColor = fontBorderColor
	c.fontBorderWidth = fontBorderWidth
	c.fontColor = &image.Uniform{C: fontColor}
	c.fontFilePath = filepath.Join("assets", "fonts", "tahomabd.ttf")
	c.fontSize = fontSize
	c.numDigits = numPlaces
	c.numPosX = numPosX
	c.numPosY = numPosY
	c.numEnd = seqEnd
	c.numStart = seqStart
	c.textImgHeight = textLayerHeight
	c.textImgWidth = textLayerWidth

	return nil
}

func (c *Config) setStaticConfigFromFlags(flags *pflag.FlagSet) error {
	bgImage, err := flags.GetString(flagNameBgImage)
	if err != nil {
		return err
	}

	c.static.bgImageFilePath = bgImage

	return nil
}

func (c *Config) setDynamicConfigFromFlags(flags *pflag.FlagSet) error {
	stillFileDirPath, err := flags.GetString(flagNameStillSrcPath)
	if err != nil {
		return err
	}
	stillFileExt, err := flags.GetString(flagNameStillFilenameExt)
	if err != nil {
		return err
	}
	stillFilePrefix, err := flags.GetString(flagNameStillFilenamePrefix)
	if err != nil {
		return err
	}
	titleImgFilePath, err := flags.GetString(flagNameTitleOverlayPath)
	if err != nil {
		return err
	}

	c.dynamic.stillSourceDirPath = stillFileDirPath
	c.dynamic.stillFilenameExt = stillFileExt
	c.dynamic.stillFilenamePrefix = stillFilePrefix
	c.dynamic.titleImageFilePath = titleImgFilePath

	return nil
}

func (c *Config) validate() error {
	var result error

	if c.numStart < 0 || c.numEnd <= 0 {
		result = multierror.Append(result, fmt.Errorf("invalid sequence: start must be a positive seqNumber"))
	}
	if c.numStart >= c.numEnd {
		result = multierror.Append(result, fmt.Errorf("invalid sequence: start seqNumber is the same as or after the end seqNumber"))
	}
	if c.numDigits < 0 {
		result = multierror.Append(result, fmt.Errorf("invalid numFixedPlaces: must be a positive seqNumber"))
	}

	return result
}

func (c *Config) validateStatic() error {
	result := c.validate()

	if c.static.bgImageFilePath == "" {
		result = multierror.Append(result, fmt.Errorf("no background baseImage file path specified"))
	}

	return result
}

func (c *Config) validateDynamic() error {
	result := c.validate()

	if c.dynamic.titleImageFilePath == "" {
		result = multierror.Append(result, fmt.Errorf("no title overlay baseImage file path specified"))
	}

	return result
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

// generateStaticThumbnails renders and saves thumbnails comprised of the given background baseImage and a sequence seqNumber
func generateStaticThumbnails() error {
	for i := conf.numStart; i <= conf.numEnd; i++ {
		thumbNail := thumbnail{
			baseImage: bgImage,
			seqNumber: i,
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

func generateDynamicThumbnails() error {
	var (
		err      error
		titleImg *image.NRGBA
	)

	titleImg = nil
	if conf.dynamic.titleImageFilePath != "" {
		titleImgPath := conf.dynamic.titleImageFilePath
		titleImg, err = importImg(titleImgPath)
		if err != nil {
			log.WithFields(log.Fields{
				"error":          err,
				"title_img.path": conf.dynamic.titleImageFilePath,
			}).Errorf("cannot open title image")
			return err
		}
	}

	for i := conf.numStart; i <= conf.numEnd; i++ {
		imgFilename := fmt.Sprintf("%s%d.%s", conf.dynamic.stillFilenamePrefix, i, conf.dynamic.stillFilenameExt)
		imgPath := path.Join(conf.dynamic.stillSourceDirPath, imgFilename)
		img, err := importImg(imgPath)
		if err != nil {
			log.WithFields(log.Fields{
				"error":        err,
				"src_img.path": imgPath,
				"seq_number":   strconv.Itoa(i),
			}).Errorf("cannot open dynamic thumbnail image")
			continue
		}
		thumbNail := thumbnail{
			baseImage:  img,
			seqNumber:  i,
			titleImage: titleImg,
		}
		if err := thumbNail.render(); err != nil {
			log.WithFields(log.Fields{
				"error":          err,
				"src_img.path":   imgPath,
				"seq_number":     strconv.Itoa(i),
				"title_img.path": conf.dynamic.titleImageFilePath,
			}).Errorf("unable to render thumbnail image")
			continue
		}
		if err := thumbNail.export(); err != nil {
			log.WithFields(log.Fields{
				"dst.path":       conf.destPath,
				"error":          err,
				"src_img.path":   imgPath,
				"seq_number":     strconv.Itoa(i),
				"title_img.path": conf.dynamic.titleImageFilePath,
			}).Errorf("unable to export thumbnail image")
			continue
		}
	}

	return nil
}

func importBackground(fpath string) error {
	nrgba, err := importImg(fpath)
	if err != nil {
		return err
	}
	bgImage = nrgba
	return nil
}

func importImg(fpath string) (nrgba *image.NRGBA, err error) {
	fileData, err := os.Open(fpath)
	if err != nil {
		return
	}

	imageData, _, err := image.Decode(fileData)
	if err != nil {
		return
	}

	nrgba = image.NewNRGBA(imageData.Bounds())
	draw.Draw(nrgba, nrgba.Bounds(), imageData, image.Point{}, draw.Src)

	return
}

func (thumb *thumbnail) setPaddedNumberFromNumber() {
	raw := strconv.Itoa(thumb.seqNumber)
	rawCharCount := strings.Count(raw, "") - 1

	if conf.numDigits <= 1 || rawCharCount >= conf.numDigits {
		// No padding required (seqNumber is longer than or equal to the seqNumber of places, so no leading 0s needed)
		thumb.paddedSeqNumber = raw
	}

	paddedNum := raw
	for i := 1; i <= conf.numDigits-rawCharCount; i++ {
		paddedNum = fmt.Sprintf("0%s", paddedNum)
	}

	log.WithFields(log.Fields{
		"conf.numDigits":                 conf.numDigits,
		"thumb.seqNumber":                thumb.seqNumber,
		"thumb.seqNumber.padded":         paddedNum,
		"thumb.seqNumber.raw_char_count": rawCharCount,
	}).Debug("setPaddedNumberFromNumber result")
	thumb.paddedSeqNumber = paddedNum
}

// render creates the baseImage and the seqNumber overlay for the thumbnail
func (thumb *thumbnail) render() error {
	// generate padded seqNumber string
	thumb.setPaddedNumberFromNumber()

	// create a new, blank baseImage
	thumb.image = image.NewNRGBA(thumb.baseImage.Bounds())

	// draw the baseImage onto the blank
	draw.Draw(thumb.image, thumb.image.Bounds(), thumb.baseImage, image.Point{}, draw.Src)

	// if there is a title image, add that layer next
	if thumb.titleImage != nil {
		log.Debug("adding title layer")
		draw.Draw(thumb.image, thumb.image.Bounds(), thumb.titleImage, image.Point{}, draw.Over)
	}

	borderColorSoft := conf.fontBorderColor
	borderColorSoft.A = 150
	borderColorSofter := conf.fontBorderColor
	borderColorSofter.A = 65

	// create a temp image to draw the text onto
	textImg := image.NewNRGBA(image.Rect(0, 0, conf.textImgWidth, conf.textImgHeight))

	// calc Y level to place the font Drawer dot at, given the font size and DPI
	// (the dot for drawing a char starts at the _bottom_ left of the char, so we need enough Y space to fit the char height)
	y := int(math.Ceil(conf.fontSize * fontDPI / 72))
	startDot := fixed.Point26_6{
		X: fixed.I(0 + 2 + conf.fontBorderWidth),
		Y: fixed.I(y + ((2 + conf.fontBorderWidth) * 2)),
	}

	textDrawer := &font.Drawer{
		Dst: textImg,
		Src: conf.fontColor,
		Face: truetype.NewFace(parsedFont, &truetype.Options{
			Size:    conf.fontSize,
			DPI:     fontDPI,
			Hinting: font.HintingFull,
		}),
		Dot: startDot,
	}
	text := fmt.Sprintf("#%v", thumb.paddedSeqNumber)
	// draw the sequence seqNumber onto the temp image
	textDrawer.DrawString(text)
	// add the main text border/outline
	imgutils.AddBorders(textImg, conf.fontBorderColor, conf.fontBorderWidth, conf.fontBorderAlphaThresh)
	// overlay the text again, for proper blending of text pixels with a non-zero alpha < 255
	textDrawer.Dot = startDot
	textDrawer.DrawString(text)
	imgutils.AddBorders(textImg, borderColorSoft, 1, conf.fontBorderAlphaThresh)
	imgutils.AddBorders(textImg, borderColorSofter, 1, 149)
	if debug {
		fileName := fmt.Sprintf("thumbnail_%s_%s_debug_textlayer.png", conf.baseName, thumb.paddedSeqNumber)
		filePath := filepath.Join(conf.destPath, fileName)
		if err := savePNG(textImg, filePath); err != nil {
			return err
		}
	}

	textRect := imgutils.OccupiedAreaRect(textImg)
	textRectAbs := image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: textRect.Size(),
	}

	// manual placement
	// destRect := textRectAbs.Bounds().Add(baseImage.Point{X: conf.numPosX, Y:conf.numPosY})

	// auto lower right corner
	// calcX := thumb.baseImage.Bounds().Dx() - textRectAbs.Bounds().Dx() - 25
	// calcY := thumb.baseImage.Bounds().Dy() - textRectAbs.Bounds().Dy() - 25

	// auto upper right corner
	calcX := thumb.image.Bounds().Dx() - textRectAbs.Bounds().Dx() - 25
	calcY := textRectAbs.Bounds().Dy() - 80
	destRect := textRectAbs.Bounds().Add(image.Point{X: calcX, Y: calcY})

	draw.Draw(thumb.image, destRect, textImg, textRect.Min, draw.Over)
	return nil
}

func (thumb *thumbnail) export() error {
	log.WithFields(log.Fields{
		"height": thumb.image.Rect.Max.Y,
		"width": thumb.image.Rect.Max.X,
	}).Debug("exporting image")
	if thumb.image.Rect.Max.X > imageFinalWidth || thumb.image.Rect.Max.Y > imageFinalHeight {
		log.WithFields(log.Fields{
			"target.height": imageFinalHeight,
			"target.width": imageFinalWidth,
		}).Debug("scaling image")
		// Scale image to fit within the required dimensions (e.g. 1280x720)
		scaledImage := image.NewNRGBA(image.Rect(0,0,imageFinalWidth,imageFinalHeight))
		draw.CatmullRom.Scale(scaledImage, scaledImage.Rect, thumb.image, thumb.image.Bounds(), draw.Over, nil)
		thumb.image = scaledImage
	}
	fileName := fmt.Sprintf("thumbnail_%s_%s.png", conf.baseName, thumb.paddedSeqNumber)
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
	log.WithFields(log.Fields{
		"dst.path": destFile,
	}).Info("saving PNG file")
	if err := png.Encode(destFh, img); err != nil {
		return err
	}
	return nil

}
