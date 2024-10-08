package routes

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/damongolding/immich-kiosk/config"
	"github.com/damongolding/immich-kiosk/immich"
	"github.com/damongolding/immich-kiosk/utils"
	"github.com/damongolding/immich-kiosk/views"
	"github.com/labstack/echo/v4"
)

// NewImage new image endpoint
func NewImage(baseConfig *config.Config) echo.HandlerFunc {
	return func(c echo.Context) error {

		if log.GetLevel() == log.DebugLevel {
			fmt.Println()
		}

		kioskVersionHeader := c.Request().Header.Get("kiosk-version")
		requestId := utils.ColorizeRequestId(c.Response().Header().Get(echo.HeaderXRequestID))

		// create a copy of the global config to use with this request
		requestConfig := *baseConfig

		// If kiosk version on client and server do not match refresh client.
		if kioskVersionHeader != "" && KioskVersion != kioskVersionHeader {
			c.Response().Header().Set("HX-Refresh", "true")
			return c.String(http.StatusTemporaryRedirect, "")
		}

		err := requestConfig.ConfigWithOverrides(c)
		if err != nil {
			log.Error("err overriding config", "err", err)
		}

		log.Debug(
			requestId,
			"method", c.Request().Method,
			"path", c.Request().URL.String(),
			"requestConfig", requestConfig.String(),
		)

		immichImage := immich.NewImage(requestConfig)

		var peopleAndAlbums []immich.ImmichAsset

		for _, people := range requestConfig.Person {
			// TODO whitelisting goes here
			peopleAndAlbums = append(peopleAndAlbums, immich.ImmichAsset{Type: "PERSON", ID: people})
		}

		for _, album := range requestConfig.Album {
			// TODO whitelisting goes here
			peopleAndAlbums = append(peopleAndAlbums, immich.ImmichAsset{Type: "ALBUM", ID: album})
		}

		pickedImage := utils.RandomItem(peopleAndAlbums)

		switch pickedImage.Type {
		case "ALBUM":
			randomAlbumImageErr := immichImage.GetRandomImageFromAlbum(pickedImage.ID, requestId)
			if randomAlbumImageErr != nil {
				log.Error("err getting image from album", "err", randomAlbumImageErr)
				return Render(c, http.StatusOK, views.Error(views.ErrorData{Title: "Error getting image from album", Message: "Is album ID correct?"}))
			}
		case "PERSON":
			randomPersonImageErr := immichImage.GetRandomImageOfPerson(pickedImage.ID, requestId)
			if randomPersonImageErr != nil {
				log.Error("err getting image of person", "err", randomPersonImageErr)
				return Render(c, http.StatusOK, views.Error(views.ErrorData{Title: "Error getting image of person", Message: "Is person ID correct?"}))
			}
		default:
			randomImageErr := immichImage.GetRandomImage(requestId)
			if randomImageErr != nil {
				log.Error("err getting random image", "err", randomImageErr)
				return Render(c, http.StatusOK, views.Error(views.ErrorData{Title: "Error getting random image", Message: "Is Immich running? Are your config settings correct?"}))
			}
		}

		imageGet := time.Now()
		imgBytes, err := immichImage.GetImagePreview()
		if err != nil {
			return err
		}
		log.Debug(requestId, "Got image in", time.Since(imageGet).Seconds())

		// if user wants the raw image (via GET request) data send it
		if c.Request().Method == http.MethodGet {
			return c.Blob(http.StatusOK, immichImage.OriginalMimeType, imgBytes)
		}

		imageConvertTime := time.Now()
		img, err := utils.ImageToBase64(imgBytes)
		if err != nil {
			return err
		}
		log.Debug(requestId, "Converted image in", time.Since(imageConvertTime).Seconds())

		var imgBlur string

		if requestConfig.BackgroundBlur && strings.ToLower(requestConfig.ImageFit) != "cover" {
			imageBlurTime := time.Now()
			imgBlurBytes, err := utils.BlurImage(imgBytes)
			if err != nil {
				log.Error("err blurring image", "err", err)
				return err
			}
			imgBlur, err = utils.ImageToBase64(imgBlurBytes)
			if err != nil {
				log.Error("err converting blurred image to base", "err", err)
				return err
			}
			log.Debug(requestId, "Blurred image in", time.Since(imageBlurTime).Seconds())
		}

		if len(requestConfig.History) > 10 {
			requestConfig.History = requestConfig.History[len(requestConfig.History)-10:]
		}

		data := views.PageData{
			ImmichImage:   immichImage,
			ImageData:     img,
			ImageBlurData: imgBlur,
			Config:        requestConfig,
		}

		return Render(c, http.StatusOK, views.Image(data))
	}
}
